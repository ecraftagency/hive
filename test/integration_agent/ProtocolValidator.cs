using Microsoft.Extensions.Logging;

namespace IntegrationAgent;

public class ProtocolValidator
{
    private readonly ILogger<ProtocolValidator> _logger;
    private readonly LogMonitor _logMonitor;

    public ProtocolValidator(ILogger<ProtocolValidator> logger, LogMonitor logMonitor)
    {
        _logger = logger;
        _logMonitor = logMonitor;
    }

    public Task<bool> ValidateGracefulShutdownAsync(string roomId, string expectedReason, TimeSpan minTime, TimeSpan maxTime)
    {
        var logs = _logMonitor.GetLogs(roomId);
        var shutdownLog = logs.FirstOrDefault(log => log.Message.Contains("Sending shutdown signal"));

        if (shutdownLog == null)
        {
            _logger.LogError("No shutdown signal found in logs for room {RoomId}", roomId);
            return Task.FromResult(false);
        }

        // Validate timing
        var elapsed = DateTime.UtcNow - shutdownLog.Timestamp;
        if (elapsed < minTime || elapsed > maxTime)
        {
            _logger.LogError("Shutdown timing out of range: {Elapsed} (expected: {MinTime}-{MaxTime})", 
                elapsed, minTime, maxTime);
            return Task.FromResult(false);
        }

        // Validate reason
        if (!shutdownLog.Message.Contains($"reason={expectedReason}"))
        {
            _logger.LogError("Shutdown reason mismatch: {Message}", shutdownLog.Message);
            return Task.FromResult(false);
        }

        _logger.LogInformation("✅ Graceful shutdown validation passed for room {RoomId}", roomId);
        return Task.FromResult(true);
    }

    public bool ValidateClientBehavior(string roomId, string[] expectedEvents)
    {
        var logs = _logMonitor.GetLogs(roomId);
        
        foreach (var expected in expectedEvents)
        {
            if (!logs.Any(log => log.Message.Contains(expected)))
            {
                _logger.LogError("Expected event not found: {Event}", expected);
                return false;
            }
        }

        _logger.LogInformation("✅ Client behavior validation passed for room {RoomId}", roomId);
        return true;
    }
}
