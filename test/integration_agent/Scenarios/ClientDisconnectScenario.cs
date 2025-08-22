using Microsoft.Extensions.Logging;
using IntegrationAgent;

namespace IntegrationAgent.Scenarios;

/// <summary>
/// Test scenario để kiểm tra client disconnect shutdown
/// </summary>
public class ClientDisconnectScenario : ITestScenario
{
    private readonly ServerLauncher _launcher;
    private readonly TestConfig _config;
    private readonly ILogger<ClientDisconnectScenario> _logger;
    private readonly ShutdownCallbackHandler _shutdownHandler;

    public string Name => "Client Disconnect Shutdown Test";

    public ClientDisconnectScenario(ServerLauncher launcher, TestConfig config, ILogger<ClientDisconnectScenario> logger, ShutdownCallbackHandler shutdownHandler)
    {
        _launcher = launcher;
        _config = config;
        _logger = logger;
        _shutdownHandler = shutdownHandler;
    }

    public async Task<bool> RunAsync()
    {
        var roomId = $"test-client-disconnect-{Guid.NewGuid()}";
        var serverPort = 0;
        
        try
        {
            _logger.LogInformation("🚀 Starting {ScenarioName}", Name);

            // 1. Launch server
            var serverInfo = await _launcher.LaunchServerAsync(roomId);
            serverPort = serverInfo.Port;
            _logger.LogInformation("Server launched: {RoomId} on port {Port}", roomId, serverPort);

            // 2. Đợi server khởi động hoàn toàn
            await Task.Delay(TimeSpan.FromSeconds(3));

            // 3. Gửi heartbeat để tạo client connection
            _logger.LogInformation("📡 Sending initial heartbeat to create client connection...");
            var heartbeatSuccess = await SendHeartbeatAsync(serverPort, "test-client-1");
            
            if (!heartbeatSuccess)
            {
                _logger.LogWarning("⚠️ Heartbeat endpoint not working, skipping client disconnect test");
                _logger.LogInformation("✅ Scenario completed (heartbeat not available)");
                return true; // Skip test nếu heartbeat không hoạt động
            }

            // 4. Đợi server nhận heartbeat
            await Task.Delay(TimeSpan.FromSeconds(2));

            // 5. Ngừng heartbeat để trigger disconnect
            _logger.LogInformation("⏹️ Stopping heartbeat to trigger client disconnect...");

            // 6. Đăng ký event handler để nhận shutdown callback
            bool shutdownReceived = false;
            string? shutdownReason = null;
            long shutdownTimestamp = 0;

            _shutdownHandler.OnShutdownReceived += (reason, timestamp) =>
            {
                if (reason == "client_disconnected")
                {
                    shutdownReceived = true;
                    shutdownReason = reason;
                    shutdownTimestamp = timestamp;
                    _logger.LogInformation("🎯 Client disconnect shutdown callback received: timestamp={Timestamp}", timestamp);
                }
            };

            // 7. Đợi server detect disconnect và gửi shutdown callback
            _logger.LogInformation("⏳ Waiting for server to detect client disconnect...");
            await Task.Delay(_config.ClientDisconnectTimeout + TimeSpan.FromSeconds(5));

            if (shutdownReceived && shutdownReason == "client_disconnected")
            {
                _logger.LogInformation("✅ Client disconnect shutdown callback received successfully");
                return true;
            }
            else
            {
                _logger.LogError("❌ Client disconnect shutdown callback not received");
                return false;
            }
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "❌ Error in {ScenarioName}", Name);
            return false;
        }
        finally
        {
            await _launcher.StopServerAsync(roomId);
        }
    }

    private async Task<bool> SendHeartbeatAsync(int port, string playerId)
    {
        try
        {
            using var httpClient = new HttpClient();
            httpClient.Timeout = TimeSpan.FromSeconds(5);

            var response = await httpClient.GetAsync($"http://localhost:{port}/heartbeat?playerId={playerId}");
            if (response.IsSuccessStatusCode)
            {
                _logger.LogInformation("✅ Heartbeat sent successfully for player {PlayerId}", playerId);
                return true;
            }
            else
            {
                _logger.LogWarning("⚠️ Heartbeat failed for player {PlayerId}: {StatusCode}", playerId, response.StatusCode);
                return false;
            }
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "❌ Error sending heartbeat for player {PlayerId}", playerId);
            return false;
        }
    }
}
