namespace IntegrationAgent;

public class TestConfig
{
    // Server launcher config
    public string ServerPath { get; set; } = "./server";
    public string ServerToken { get; set; } = "test-token";
    
    // Server flags
    public bool UseNoGraphics { get; set; } = true;  // Default: true cho test
    public bool UseBatchMode { get; set; } = true;   // Default: true cho test
    
    // Test agent config
    public int AgentPort { get; set; } = 8081;
    public string AgentBaseUrl { get; set; } = "http://localhost:8081";
    
    // Test timeouts
    public TimeSpan GraceShutdownTimeout { get; set; } = TimeSpan.FromSeconds(20);
    public TimeSpan ClientDisconnectTimeout { get; set; } = TimeSpan.FromSeconds(10);
    public TimeSpan HeartbeatInterval { get; set; } = TimeSpan.FromSeconds(3);
    
    // Validation windows
    public TimeSpan MinShutdownTime { get; set; } = TimeSpan.FromSeconds(18);
    public TimeSpan MaxShutdownTime { get; set; } = TimeSpan.FromSeconds(22);
    
    // Logging
    public string LogLevel { get; set; } = "Information";
}
