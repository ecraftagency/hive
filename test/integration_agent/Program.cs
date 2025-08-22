using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Configuration;
using IntegrationAgent;
using IntegrationAgent.Scenarios;

var builder = Host.CreateDefaultBuilder(args)
    .ConfigureServices((context, services) =>
    {
        // Configure services
        services.AddSingleton<TestConfig>(provider =>
        {
            var config = new TestConfig();
            context.Configuration.GetSection("TestConfig").Bind(config);
            return config;
        });

        services.AddSingleton<LogMonitor>();
        services.AddSingleton<ServerLauncher>();
        services.AddSingleton<ShutdownCallbackHandler>();

        // Register scenarios
        services.AddTransient<NoClientsScenario>();
        services.AddTransient<ShutdownCallbackScenario>();
        services.AddTransient<ClientDisconnectScenario>();
        services.AddTransient<SignalShutdownScenario>();
    });

var host = builder.Build();

// Validate port conflict
var config = host.Services.GetRequiredService<TestConfig>();
var logger = host.Services.GetRequiredService<ILogger<Program>>();

if (config.AgentPort == 8080)
{
    logger.LogError("❌ ERROR: Test agent port cannot be 8080 (conflicts with real agent)");
    logger.LogError("Please use a different port (e.g., 8081, 8082, etc.)");
    return;
}

logger.LogInformation("✅ Test agent will run on port {AgentPort}", config.AgentPort);
logger.LogInformation("✅ Server will send signals to {AgentBaseUrl}", config.AgentBaseUrl);

// Khởi tạo shutdown callback handler
var shutdownHandler = host.Services.GetRequiredService<ShutdownCallbackHandler>();
shutdownHandler.OnShutdownReceived += (reason, timestamp) =>
{
    var reasonDescription = reason switch
    {
        "no_clients" => "No clients in grace period",
        "client_disconnected" => "Client disconnected",
        "signal_received" => "Signal shutdown (SIGINT/SIGTERM)",
        "afk_timeout" => "Client AFK timeout",
        "game_cycle_completed" => "Game cycle completed",
        _ => "Unknown reason"
    };
    
    logger.LogInformation("🎯 SHUTDOWN SIGNAL RECEIVED: {Reason} ({Description}) at {Timestamp}", 
        reason, reasonDescription, timestamp);
    
    // Có thể thêm logic xử lý shutdown ở đây
    // Ví dụ: lưu log, gửi notification, etc.
};

// Run tests
var scenarios = new ITestScenario[]
{
    host.Services.GetRequiredService<NoClientsScenario>(),
    host.Services.GetRequiredService<ShutdownCallbackScenario>(),
    host.Services.GetRequiredService<ClientDisconnectScenario>(),
    host.Services.GetRequiredService<SignalShutdownScenario>()
};

logger.LogInformation("Starting integration tests...");

try
{
    foreach (var scenario in scenarios)
    {
        logger.LogInformation("Running scenario: {ScenarioName}", scenario.Name);
        
        var success = await scenario.RunAsync();
        
        if (success)
        {
            logger.LogInformation("✅ Scenario {ScenarioName} passed", scenario.Name);
        }
        else
        {
            logger.LogError("❌ Scenario {ScenarioName} failed", scenario.Name);
        }
    }
}
finally
{
    // Cleanup
    shutdownHandler.Dispose();
}

logger.LogInformation("Integration tests completed");
