using Microsoft.Extensions.Logging;
using System.Diagnostics;
using System.Net;
using System.Net.Sockets;

namespace IntegrationAgent;

public class ServerLauncher
{
    private readonly TestConfig _config;
    private readonly ILogger<ServerLauncher> _logger;
    private readonly Dictionary<string, ServerProcess> _processes = new();
    private readonly LogMonitor _logMonitor;

    public ServerLauncher(TestConfig config, ILogger<ServerLauncher> logger, LogMonitor logMonitor)
    {
        _config = config;
        _logger = logger;
        _logMonitor = logMonitor;
    }

    public async Task<ServerInfo> LaunchServerAsync(string roomId)
    {
        // 1. Find available port
        var port = await FindAvailablePortAsync(9090, 9999);
        
        // 2. Build command arguments với đầy đủ flags
        var args = new List<string>
        {
            "-port", port.ToString(),
            "-serverId", roomId,
            "-token", _config.ServerToken
        };

        // Thêm optional flags nếu được config
        if (_config.UseNoGraphics)
        {
            args.Add("-nographics");
        }
        
        if (_config.UseBatchMode)
        {
            args.Add("-batchmode");
        }

        // 3. Set environment variables
        var envVars = new Dictionary<string, string>
        {
            ["AGENT_BASE_URL"] = _config.AgentBaseUrl
        };

        // 4. Create process
        var process = new Process
        {
            StartInfo = new ProcessStartInfo
            {
                FileName = _config.ServerPath,
                Arguments = string.Join(" ", args),
                UseShellExecute = false,
                RedirectStandardOutput = true,
                RedirectStandardError = true,
                CreateNoWindow = true
            }
        };

        // Set environment variables
        foreach (var env in envVars)
        {
            process.StartInfo.EnvironmentVariables[env.Key] = env.Value;
        }
        
        // Debug: log environment variables
        _logger.LogInformation("Environment variables set:");
        foreach (System.Collections.DictionaryEntry env in process.StartInfo.EnvironmentVariables)
        {
            _logger.LogInformation("  {Key}={Value}", env.Key, env.Value);
        }

        _logger.LogInformation("Launching server: {ServerPath} {Arguments}", 
            _config.ServerPath, string.Join(" ", args));
        _logger.LogInformation("Environment: AGENT_BASE_URL={AgentUrl}", _config.AgentBaseUrl);

        // 5. Start process
        if (!process.Start())
        {
            throw new InvalidOperationException("Failed to start server process");
        }

        // 6. Start log monitoring
        _logMonitor.StartMonitoring(process, roomId);

        // 7. Store process info
        var serverProcess = new ServerProcess
        {
            RoomId = roomId,
            Process = process,
            Port = port,
            StartTime = DateTime.UtcNow
        };

        _processes[roomId] = serverProcess;

        // 8. Wait for server to start
        await WaitForServerStartAsync(roomId, TimeSpan.FromSeconds(10));

        return new ServerInfo
        {
            RoomId = roomId,
            Port = port,
            ProcessId = process.Id,
            StartTime = DateTime.UtcNow
        };
    }

    public async Task StopServerAsync(string roomId)
    {
        if (_processes.TryGetValue(roomId, out var serverProcess))
        {
            try
            {
                if (!serverProcess.Process.HasExited)
                {
                    serverProcess.Process.Kill();
                    await serverProcess.Process.WaitForExitAsync();
                }
            }
            catch (Exception ex)
            {
                _logger.LogWarning(ex, "Error stopping server {RoomId}", roomId);
            }
            finally
            {
                _processes.Remove(roomId);
            }
        }
    }

    private Task<int> FindAvailablePortAsync(int minPort, int maxPort)
    {
        for (int port = minPort; port <= maxPort; port++)
        {
            try
            {
                var listener = new TcpListener(IPAddress.Loopback, port);
                listener.Start();
                listener.Stop();
                return Task.FromResult(port);
            }
            catch
            {
                continue;
            }
        }
        throw new InvalidOperationException("No available ports found");
    }

    private async Task WaitForServerStartAsync(string roomId, TimeSpan timeout)
    {
        var deadline = DateTime.UtcNow.Add(timeout);
        
        while (DateTime.UtcNow < deadline)
        {
            var process = _processes[roomId];
            if (process.Process.HasExited)
            {
                throw new InvalidOperationException("Server process exited unexpectedly");
            }

            // Check logs for "Server listening on :{port}" message
            var logs = _logMonitor.GetLogs(roomId);
            if (logs.Any(log => log.Message.Contains("Server listening on :")))
            {
                _logger.LogInformation("Server started successfully: {RoomId} on port {Port}", 
                    roomId, process.Port);
                return;
            }

            await Task.Delay(100);
        }

        throw new TimeoutException("Server failed to start within timeout");
    }
}

public class ServerProcess
{
    public string RoomId { get; set; } = string.Empty;
    public Process Process { get; set; } = null!;
    public DateTime StartTime { get; set; }
    public int Port { get; set; }
}
