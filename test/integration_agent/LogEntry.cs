namespace IntegrationAgent;

public class LogEntry
{
    public DateTime Timestamp { get; set; }
    public string Message { get; set; } = string.Empty;
    public string Source { get; set; } = string.Empty; // "stdout" hoặc "stderr"
    public string Level { get; set; } = "INFO"; // "INFO", "WARN", "ERROR"
}
