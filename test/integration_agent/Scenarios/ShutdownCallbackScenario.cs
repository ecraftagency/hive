using Microsoft.Extensions.Logging;
using IntegrationAgent;

namespace IntegrationAgent.Scenarios;

/// <summary>
/// Test scenario để kiểm tra shutdown callback từ server
/// </summary>
public class ShutdownCallbackScenario : ITestScenario
{
    private readonly ILogger<ShutdownCallbackScenario> _logger;
    private readonly TestConfig _config;
    private readonly ShutdownCallbackHandler _shutdownHandler;
    
    public string Name => "Shutdown Callback Test";
    
    // Tất cả các loại shutdown reason từ server
    private readonly string[] _expectedReasons = new[]
    {
        "no_clients",           // Không có client trong grace period
        "client_disconnected",  // Client bị disconnect
        "signal_received",      // Nhận signal shutdown (SIGINT/SIGTERM)
        "afk_timeout",          // Client AFK timeout
        "game_cycle_completed"  // Game cycle hoàn thành
    };
    
    public ShutdownCallbackScenario(ILogger<ShutdownCallbackScenario> logger, TestConfig config, ShutdownCallbackHandler shutdownHandler)
    {
        _logger = logger;
        _config = config;
        _shutdownHandler = shutdownHandler;
    }
    
    public async Task<bool> RunAsync()
    {
        _logger.LogInformation("🚀 Starting {ScenarioName}", Name);
        
        try
        {
            // Đợi một chút để đảm bảo HTTP listener đã sẵn sàng
            await Task.Delay(TimeSpan.FromSeconds(2));
            
            // Test 1: Kiểm tra HTTP listener có đang lắng nghe không
            _logger.LogInformation("📡 Testing HTTP listener on port {Port}", _config.AgentPort);
            
            using var httpClient = new HttpClient();
            httpClient.Timeout = TimeSpan.FromSeconds(5);
            
            try
            {
                var response = await httpClient.GetAsync($"http://localhost:{_config.AgentPort}/health");
                _logger.LogInformation("✅ HTTP listener is responding (status: {StatusCode})", response.StatusCode);
            }
            catch (Exception ex)
            {
                _logger.LogWarning("⚠️ HTTP listener test failed: {Message}", ex.Message);
            }
            
            // Test 1.5: Kiểm tra server heartbeat endpoint (nếu có server đang chạy)
            _logger.LogInformation("📡 Testing server heartbeat endpoint...");
            await TestServerHeartbeatAsync(httpClient);
            
            // Test 2: Test tất cả các loại shutdown callback
            _logger.LogInformation("🧪 Testing all shutdown callback reasons...");
            
            var allTestsPassed = true;
            
            foreach (var reason in _expectedReasons)
            {
                var testPassed = await TestShutdownCallbackAsync(httpClient, reason);
                if (!testPassed)
                {
                    allTestsPassed = false;
                    _logger.LogError("❌ Test failed for reason: {Reason}", reason);
                }
                else
                {
                    _logger.LogInformation("✅ Test passed for reason: {Reason}", reason);
                }
                
                // Đợi một chút giữa các test
                await Task.Delay(TimeSpan.FromSeconds(1));
            }
            
            return allTestsPassed;
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "❌ Error in {ScenarioName}", Name);
            return false;
        }
    }
    
    private async Task<bool> TestShutdownCallbackAsync(HttpClient httpClient, string reason)
    {
        try
        {
            _logger.LogInformation("🧪 Testing shutdown callback with reason: {Reason}", reason);
            
            // Đăng ký event handler cho test này
            bool shutdownReceived = false;
            string? receivedReason = null;
            long receivedTimestamp = 0;
            
            _shutdownHandler.OnShutdownReceived += (callbackReason, timestamp) =>
            {
                if (callbackReason == reason) // Chỉ xử lý event cho reason đang test
                {
                    shutdownReceived = true;
                    receivedReason = callbackReason;
                    receivedTimestamp = timestamp;
                    _logger.LogInformation("🎯 Shutdown event received for {Reason}: timestamp={Timestamp}", reason, timestamp);
                }
            };
            
            // Gửi test shutdown callback
            var testRequest = new ShutdownRequest
            {
                Reason = reason,
                At = DateTimeOffset.Now.ToUnixTimeSeconds()
            };
            
            var jsonContent = System.Text.Json.JsonSerializer.Serialize(testRequest);
            var content = new StringContent(jsonContent, System.Text.Encoding.UTF8, "application/json");
            
            var shutdownResponse = await httpClient.PostAsync($"http://localhost:{_config.AgentPort}/rooms/test-room/shutdown", content);
            
            if (shutdownResponse.IsSuccessStatusCode)
            {
                var responseContent = await shutdownResponse.Content.ReadAsStringAsync();
                _logger.LogInformation("✅ Test shutdown callback sent successfully for {Reason}: {Response}", reason, responseContent);
                
                // Đợi một chút để event được xử lý
                await Task.Delay(TimeSpan.FromSeconds(1));
                
                if (shutdownReceived && receivedReason == reason)
                {
                    _logger.LogInformation("✅ Shutdown event received successfully for {Reason}: timestamp={Timestamp}", 
                        receivedReason, receivedTimestamp);
                    return true;
                }
                else
                {
                    _logger.LogError("❌ Shutdown event not received or wrong reason for {Reason}", reason);
                    return false;
                }
            }
            else
            {
                _logger.LogError("❌ Failed to send test shutdown callback for {Reason}: {StatusCode}", reason, shutdownResponse.StatusCode);
                return false;
            }
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "❌ Error testing shutdown callback for {Reason}", reason);
            return false;
        }
    }

    private async Task TestServerHeartbeatAsync(HttpClient httpClient)
    {
        try
        {
            // Test các port phổ biến để tìm server
            var commonPorts = new[] { 9090, 8080, 3000, 5000 };
            
            foreach (var port in commonPorts)
            {
                try
                {
                    var response = await httpClient.GetAsync($"http://localhost:{port}/heartbeat?playerId=test-heartbeat");
                    if (response.IsSuccessStatusCode)
                    {
                        _logger.LogInformation("✅ Server heartbeat endpoint found on port {Port}", port);
                        return;
                    }
                    else
                    {
                        _logger.LogDebug("Server on port {Port} responded with status: {StatusCode}", port, response.StatusCode);
                    }
                }
                catch (Exception ex)
                {
                    _logger.LogDebug("No server on port {Port}: {Message}", port, ex.Message);
                }
            }
            
            _logger.LogInformation("ℹ️ No active server heartbeat endpoint found");
        }
        catch (Exception ex)
        {
            _logger.LogWarning("⚠️ Error testing server heartbeat: {Message}", ex.Message);
        }
    }
}
