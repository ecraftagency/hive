namespace IntegrationAgent.Scenarios;

public interface ITestScenario
{
    string Name { get; }
    Task<bool> RunAsync();
}
