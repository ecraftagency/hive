namespace IntegrationAgent;

public class ServerInfo
{
    public string RoomId { get; set; } = string.Empty;
    public int Port { get; set; }
    public int ProcessId { get; set; }
    public DateTime StartTime { get; set; }
}
