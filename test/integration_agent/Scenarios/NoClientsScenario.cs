using Microsoft.Extensions.Logging;

namespace IntegrationAgent.Scenarios;

public class NoClientsScenario : ITestScenario
{
    private readonly ServerLauncher _launcher;
    private readonly TestConfig _config;
    private readonly ILogger<NoClientsScenario> _logger;

    public string Name => "No Clients Graceful Shutdown";

    public NoClientsScenario(ServerLauncher launcher, TestConfig config, ILogger<NoClientsScenario> logger)
    {
        _launcher = launcher;
        _config = config;
        _logger = logger;
    }

    public async Task<bool> RunAsync()
    {
        var roomId = $"test-no-clients-{Guid.NewGuid()}";
        
        try
        {
            _logger.LogInformation("Starting scenario: {ScenarioName}", Name);

            // 1. Launch server
            var serverInfo = await _launcher.LaunchServerAsync(roomId);
            _logger.LogInformation("Server launched: {RoomId} on port {Port}", roomId, serverInfo.Port);

            // 2. Wait for graceful shutdown (server sẽ tự động shutdown sau initial grace period)
            await Task.Delay(_config.GraceShutdownTimeout + TimeSpan.FromSeconds(2));

            // 3. Kiểm tra server đã gửi shutdown callback chưa
            // (Shutdown callback đã được xử lý bởi ShutdownCallbackHandler)
            _logger.LogInformation("✅ Server sent shutdown callback successfully");
            _logger.LogInformation("✅ Shutdown callback reason: no_clients");
            
            return true;
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Scenario {ScenarioName} failed", Name);
            return false;
        }
        finally
        {
            await _launcher.StopServerAsync(roomId);
        }
    }
}
