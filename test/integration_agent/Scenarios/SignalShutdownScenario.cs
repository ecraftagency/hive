using Microsoft.Extensions.Logging;
using IntegrationAgent;
using System.Diagnostics;

namespace IntegrationAgent.Scenarios;

/// <summary>
/// Test scenario để kiểm tra signal shutdown (SIGINT/SIGTERM)
/// </summary>
public class SignalShutdownScenario : ITestScenario
{
    private readonly ServerLauncher _launcher;
    private readonly TestConfig _config;
    private readonly ILogger<SignalShutdownScenario> _logger;
    private readonly ShutdownCallbackHandler _shutdownHandler;

    public string Name => "Signal Shutdown Test";

    public SignalShutdownScenario(ServerLauncher launcher, TestConfig config, ILogger<SignalShutdownScenario> logger, ShutdownCallbackHandler shutdownHandler)
    {
        _launcher = launcher;
        _config = config;
        _logger = logger;
        _shutdownHandler = shutdownHandler;
    }

    public async Task<bool> RunAsync()
    {
        var roomId = $"test-signal-shutdown-{Guid.NewGuid()}";
        Process? serverProcess = null;
        
        try
        {
            _logger.LogInformation("🚀 Starting {ScenarioName}", Name);

            // 1. Launch server
            var serverInfo = await _launcher.LaunchServerAsync(roomId);
            _logger.LogInformation("Server launched: {RoomId} on port {Port}", roomId, serverInfo.Port);

            // 2. Đợi server khởi động hoàn toàn
            await Task.Delay(TimeSpan.FromSeconds(3));

            // 3. Gửi heartbeat để tạo client connection
            _logger.LogInformation("📡 Sending heartbeat to create client connection...");
            await SendHeartbeatAsync(serverInfo.Port, "test-client-1");

            // 4. Đợi server nhận heartbeat
            await Task.Delay(TimeSpan.FromSeconds(2));

            // 5. Đăng ký event handler để nhận shutdown callback
            bool shutdownReceived = false;
            string? shutdownReason = null;
            long shutdownTimestamp = 0;

            _shutdownHandler.OnShutdownReceived += (reason, timestamp) =>
            {
                if (reason == "signal_received")
                {
                    shutdownReceived = true;
                    shutdownReason = reason;
                    shutdownTimestamp = timestamp;
                    _logger.LogInformation("🎯 Signal shutdown callback received: timestamp={Timestamp}", timestamp);
                }
            };

            // 6. Gửi SIGINT signal đến server process
            _logger.LogInformation("📡 Sending SIGINT signal to server process...");
            
            // Tìm server process
            serverProcess = FindServerProcessAsync(serverInfo.Port);
            if (serverProcess != null)
            {
                _logger.LogInformation("Found server process: PID={Pid}", serverProcess.Id);
                
                // Gửi SIGINT signal bằng kill command
                await SendSignalToProcessAsync(serverProcess.Id, "SIGINT");
                _logger.LogInformation("✅ SIGINT signal sent to server process");
                
                // Đợi một chút để server xử lý signal
                await Task.Delay(TimeSpan.FromSeconds(3));
            }
            else
            {
                _logger.LogWarning("⚠️ Could not find server process for port {Port}", serverInfo.Port);
            }

            // 7. Đợi server xử lý signal và gửi shutdown callback
            _logger.LogInformation("⏳ Waiting for server to process signal and send shutdown callback...");
            await Task.Delay(TimeSpan.FromSeconds(10));

            if (shutdownReceived && shutdownReason == "signal_received")
            {
                _logger.LogInformation("✅ Signal shutdown callback received successfully");
                return true;
            }
            else
            {
                _logger.LogError("❌ Signal shutdown callback not received");
                
                // Kiểm tra xem server có còn chạy không
                if (serverProcess != null && !serverProcess.HasExited)
                {
                    _logger.LogWarning("⚠️ Server still running, trying to force shutdown...");
                    try
                    {
                        serverProcess.Kill();
                        _logger.LogInformation("Server process force killed");
                    }
                    catch (Exception ex)
                    {
                        _logger.LogWarning("Could not force kill server: {Message}", ex.Message);
                    }
                }
                
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
            // Cleanup
            if (serverProcess != null && !serverProcess.HasExited)
            {
                try
                {
                    serverProcess.Kill();
                    _logger.LogInformation("Server process killed");
                }
                catch (Exception ex)
                {
                    _logger.LogWarning("Could not kill server process: {Message}", ex.Message);
                }
            }
            
            await _launcher.StopServerAsync(roomId);
        }
    }

    private async Task SendSignalToProcessAsync(int processId, string signal)
    {
        try
        {
            var startInfo = new ProcessStartInfo
            {
                FileName = "kill",
                Arguments = $"-{signal} {processId}",
                UseShellExecute = false,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                CreateNoWindow = true
            };

            using var killProcess = Process.Start(startInfo);
            if (killProcess != null)
            {
                await killProcess.WaitForExitAsync();
                var output = await killProcess.StandardOutput.ReadToEndAsync();
                var error = await killProcess.StandardError.ReadToEndAsync();
                
                if (killProcess.ExitCode == 0)
                {
                    _logger.LogInformation("✅ Signal {Signal} sent to process {ProcessId}", signal, processId);
                }
                else
                {
                    _logger.LogWarning("⚠️ Failed to send signal {Signal} to process {ProcessId}: {Error}", signal, processId, error);
                }
            }
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "❌ Error sending signal {Signal} to process {ProcessId}", signal, processId);
        }
    }

    private Process? FindServerProcessAsync(int port)
    {
        try
        {
            // Tìm process đang listen trên port
            var processes = Process.GetProcesses();
            
            foreach (var process in processes)
            {
                try
                {
                    // Kiểm tra xem process có đang listen trên port không
                    if (IsProcessListeningOnPort(process, port))
                    {
                        return process;
                    }
                }
                catch
                {
                    // Bỏ qua process không thể kiểm tra
                    continue;
                }
            }
            
            return null;
        }
        catch (Exception ex)
        {
            _logger.LogWarning("Could not find server process: {Message}", ex.Message);
            return null;
        }
    }

    private bool IsProcessListeningOnPort(Process process, int port)
    {
        try
        {
            // Sử dụng lsof để kiểm tra port
            var startInfo = new ProcessStartInfo
            {
                FileName = "lsof",
                Arguments = $"-i :{port} -t",
                UseShellExecute = false,
                RedirectStandardOutput = true,
                CreateNoWindow = true
            };

            using var lsofProcess = Process.Start(startInfo);
            if (lsofProcess != null)
            {
                var output = lsofProcess.StandardOutput.ReadToEnd();
                lsofProcess.WaitForExit();
                
                if (!string.IsNullOrEmpty(output))
                {
                    var pids = output.Trim().Split('\n');
                    foreach (var pid in pids)
                    {
                        if (int.TryParse(pid, out var processId) && processId == process.Id)
                        {
                            return true;
                        }
                    }
                }
            }
            
            return false;
        }
        catch
        {
            return false;
        }
    }

    private async Task SendHeartbeatAsync(int port, string playerId)
    {
        try
        {
            using var httpClient = new HttpClient();
            httpClient.Timeout = TimeSpan.FromSeconds(5);

            var response = await httpClient.GetAsync($"http://localhost:{port}/heartbeat?playerId={playerId}");
            if (response.IsSuccessStatusCode)
            {
                _logger.LogInformation("✅ Heartbeat sent successfully for player {PlayerId}", playerId);
            }
            else
            {
                _logger.LogWarning("⚠️ Heartbeat failed for player {PlayerId}: {StatusCode}", playerId, response.StatusCode);
            }
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "❌ Error sending heartbeat for player {PlayerId}", playerId);
        }
    }

    private async Task SendSignalViaHttpAsync(int port)
    {
        try
        {
            // Gửi POST request để trigger signal shutdown
            using var httpClient = new HttpClient();
            httpClient.Timeout = TimeSpan.FromSeconds(5);

            var response = await httpClient.PostAsync($"http://localhost:{port}/game/trigger-shutdown", null);
            if (response.IsSuccessStatusCode)
            {
                _logger.LogInformation("✅ Signal shutdown triggered via HTTP");
            }
            else
            {
                _logger.LogWarning("⚠️ HTTP trigger shutdown failed: {StatusCode}", response.StatusCode);
            }
        }
        catch (Exception ex)
        {
            _logger.LogWarning("⚠️ HTTP trigger shutdown not available: {Message}", ex.Message);
        }
    }
}
