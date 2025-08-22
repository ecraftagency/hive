using System.Text.RegularExpressions;
using Microsoft.Extensions.Logging;
using System.Diagnostics;

namespace IntegrationAgent;

public class LogMonitor
{
    private readonly ILogger<LogMonitor> _logger;
    private readonly Dictionary<string, List<LogEntry>> _logs = new();
    private readonly Dictionary<string, Regex> _patterns = new();

    public LogMonitor(ILogger<LogMonitor> logger)
    {
        _logger = logger;
        InitializePatterns();
    }

    private void InitializePatterns()
    {
        // Server startup patterns
        _patterns["server_started"] = new Regex(@"Server listening on :(\d+) room=(\w+)");
        _patterns["server_starting"] = new Regex(@"Starting CS Game Server:");
        
        // Shutdown patterns
        _patterns["shutdown_signal"] = new Regex(@"Sending shutdown signal.*reason=(\w+)");
        _patterns["graceful_shutdown"] = new Regex(@"Graceful shutdown.*reason=(\w+)");
        _patterns["signal_shutdown"] = new Regex(@"Received Ctrl\+C, initiating graceful shutdown");
        
        // Client patterns
        _patterns["client_connected"] = new Regex(@"Client connected.*player_id=(\w+)");
        _patterns["client_disconnected"] = new Regex(@"Client disconnected.*player_id=(\w+)");
        _patterns["heartbeat_received"] = new Regex(@"Heartbeat from (\w+)");
        _patterns["no_clients"] = new Regex(@"No clients detected");
        
        // Error patterns
        _patterns["server_error"] = new Regex(@"Server error: (.+)");
        _patterns["game_finish"] = new Regex(@"Game finish");
    }

    public void StartMonitoring(Process process, string roomId)
    {
        _logs[roomId] = new List<LogEntry>();

        // Monitor stdout
        process.OutputDataReceived += (sender, e) =>
        {
            if (e.Data != null)
            {
                AddLogEntry(roomId, e.Data, "stdout");
            }
        };

        // Monitor stderr
        process.ErrorDataReceived += (sender, e) =>
        {
            if (e.Data != null)
            {
                AddLogEntry(roomId, e.Data, "stderr");
            }
        };

        process.BeginOutputReadLine();
        process.BeginErrorReadLine();
    }

    private void AddLogEntry(string roomId, string message, string source)
    {
        var entry = new LogEntry
        {
            Timestamp = DateTime.UtcNow,
            Message = message,
            Source = source,
            Level = DetermineLogLevel(message)
        };

        lock (_logs)
        {
            if (_logs.ContainsKey(roomId))
            {
                _logs[roomId].Add(entry);
            }
        }

        _logger.LogDebug("[{RoomId}] {Source}: {Message}", roomId, source, message);
        CheckPatterns(roomId, entry);
    }

    private string DetermineLogLevel(string message)
    {
        if (message.Contains("ERROR", StringComparison.OrdinalIgnoreCase) || 
            message.Contains("Server error:", StringComparison.OrdinalIgnoreCase))
            return "ERROR";
        if (message.Contains("WARN", StringComparison.OrdinalIgnoreCase))
            return "WARN";
        return "INFO";
    }

    private void CheckPatterns(string roomId, LogEntry entry)
    {
        foreach (var pattern in _patterns)
        {
            var match = pattern.Value.Match(entry.Message);
            if (match.Success)
            {
                _logger.LogInformation("[{RoomId}] Pattern matched: {Pattern} = {Value}", 
                    roomId, pattern.Key, match.Groups.Count > 1 ? match.Groups[1].Value : "matched");
            }
        }
    }

    public List<LogEntry> GetLogs(string roomId)
    {
        lock (_logs)
        {
            return _logs.TryGetValue(roomId, out var logs) ? logs.ToList() : new List<LogEntry>();
        }
    }
}
