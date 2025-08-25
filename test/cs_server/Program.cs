using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Mvc;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using Newtonsoft.Json;
using System.Collections.Concurrent;
using System.Net;
using System.Text;
using System.Runtime.Loader;

namespace CsServer
{
    /// <summary>
    /// Player information with connection status
    /// This structure must match exactly what the Agent expects for consistency
    /// </summary>
    public class PlayerInfo
    {
        /// <summary>
        /// Unique identifier for the player
        /// </summary>
        public string PlayerId { get; set; } = string.Empty;
        
        /// <summary>
        /// Current connection state: "connected" or "disconnected"
        /// Based on heartbeat activity within the TTL window
        /// </summary>
        public string State { get; set; } = "connected";
        
        /// <summary>
        /// Unix timestamp of last heartbeat received
        /// Used by Agent to determine player activity
        /// </summary>
        public long LastSeenUnix { get; set; }
    }

    /// <summary>
    /// Shutdown callback request body sent to Agent
    /// Must match the exact format expected by the Agent's shutdown endpoint
    /// </summary>
    public class ShutdownRequest
    {
        /// <summary>
        /// Reason for shutdown - must be one of the valid values accepted by Agent
        /// Valid values: "no_clients", "client_disconnected", "afk_timeout", "game_cycle_completed", "signal_received"
        /// </summary>
        public string Reason { get; set; } = string.Empty;
        
        /// <summary>
        /// Unix timestamp when shutdown was initiated
        /// Optional but recommended for audit purposes
        /// </summary>
        public long At { get; set; }
    }

    /// <summary>
    /// Response from Agent shutdown callback
    /// Used to confirm successful communication with Agent
    /// </summary>
    public class ShutdownResponse
    {
        /// <summary>
        /// Whether the shutdown callback was processed successfully
        /// </summary>
        public bool Ok { get; set; }
        
        /// <summary>
        /// Status message from Agent
        /// </summary>
        public string Status { get; set; } = string.Empty;
    }

    /// <summary>
    /// Manages player list and heartbeat tracking
    /// Uses thread-safe collections for concurrent access
    /// </summary>
    public class PlayerStore
    {
        /// <summary>
        /// Thread-safe dictionary tracking last heartbeat time for each player
        /// Key: PlayerId, Value: Last heartbeat timestamp
        /// </summary>
        private readonly ConcurrentDictionary<string, DateTime> _lastSeen = new();
        
        private readonly ILogger<PlayerStore> _logger;

        public PlayerStore(ILogger<PlayerStore> logger)
        {
            _logger = logger;
        }

        /// <summary>
        /// Records heartbeat from a player
        /// Updates the last seen timestamp for tracking connection status
        /// </summary>
        /// <param name="playerId">Unique identifier for the player</param>
        public void Heartbeat(string playerId)
        {
            _lastSeen.AddOrUpdate(playerId, DateTime.Now, (_, _) => DateTime.Now);
            _logger.LogInformation("Heartbeat from {PlayerId}", playerId);
        }

        /// <summary>
        /// Current number of active players
        /// </summary>
        public int Size => _lastSeen.Count;

        /// <summary>
        /// Creates a snapshot of player list with current connection status
        /// Used by the /players endpoint to provide real-time player information
        /// </summary>
        /// <param name="ttl">Time-to-live window for considering a player connected</param>
        /// <returns>List of player information with connection status</returns>
        public List<PlayerInfo> Snapshot(TimeSpan ttl)
        {
            var now = DateTime.Now;
            var result = new List<PlayerInfo>();

            foreach (var kvp in _lastSeen)
            {
                // Player is considered connected if heartbeat received within TTL window
                var state = (now - kvp.Value) <= ttl ? "connected" : "disconnected";
                result.Add(new PlayerInfo
                {
                    PlayerId = kvp.Key,
                    State = state,
                    LastSeenUnix = new DateTimeOffset(kvp.Value).ToUnixTimeSeconds()
                });
            }

            return result;
        }

        /// <summary>
        /// Checks if any player has been disconnected beyond the TTL window
        /// Used for graceful shutdown detection
        /// </summary>
        /// <param name="ttl">Time-to-live window for connection status</param>
        /// <returns>Tuple indicating if any player is disconnected and their ID</returns>
        public (bool hasDisconnected, string? playerId) AnyDisconnected(TimeSpan ttl)
        {
            var now = DateTime.Now;
            foreach (var kvp in _lastSeen)
            {
                if ((now - kvp.Value) > ttl)
                {
                    return (true, kvp.Key);
                }
            }
            return (false, null);
        }
    }

    /// <summary>
    /// Main game server implementation
    /// Handles player monitoring, graceful shutdown, and Agent communication
    /// Must implement the exact protocol expected by the Hive Agent
    /// </summary>
    public class GameServer
    {
        private readonly ILogger<GameServer> _logger;
        private readonly PlayerStore _players;
        private readonly string _roomId;
        private readonly string _bearerToken;
        private readonly string _agentBaseUrl;
        
        /// <summary>
        /// Time window for considering a player connected
        /// Must be shorter than Agent's allocation timeout to ensure proper cleanup
        /// </summary>
        private readonly TimeSpan _heartbeatTtl = TimeSpan.FromSeconds(10);
        
        /// <summary>
        /// Initial grace period before starting monitoring
        /// Allows server to fully start up before checking player activity
        /// </summary>
        private readonly TimeSpan _initialGrace = TimeSpan.FromSeconds(20);
        
        /// <summary>
        /// Cancellation token source for graceful shutdown
        /// Used to coordinate shutdown across all components
        /// </summary>
        private readonly CancellationTokenSource _shutdownCts = new();
        
        /// <summary>
        /// HTTP client for Agent communication
        /// Configured with timeout for reliable shutdown callbacks
        /// </summary>
        private readonly HttpClient _httpClient = new();

        public GameServer(ILogger<GameServer> logger, ILogger<PlayerStore> playerLogger, string roomId, string bearerToken, string agentBaseUrl)
        {
            _logger = logger;
            _players = new PlayerStore(playerLogger);
            _roomId = roomId;
            _bearerToken = bearerToken;
            _agentBaseUrl = agentBaseUrl;
            
            // Set reasonable timeout for Agent communication
            // Must be shorter than Agent's shutdown processing time
            _httpClient.Timeout = TimeSpan.FromSeconds(5);
        }

        /// <summary>
        /// Sends shutdown callback to Agent
        /// This is the critical integration point with the Hive Agent
        /// Must use the exact endpoint and format specified in Agent documentation
        /// </summary>
        /// <param name="reason">Shutdown reason - must match Agent's expected values</param>
        private async Task SendShutdownCallbackAsync(string reason)
        {
            // Validate required parameters before sending callback
            if (string.IsNullOrEmpty(_roomId) || string.IsNullOrEmpty(_bearerToken) || string.IsNullOrEmpty(_agentBaseUrl))
            {
                _logger.LogWarning("Shutdown callback skipped: roomId={RoomId}, bearer={Bearer}, agentBase={AgentBase}", 
                    _roomId, _bearerToken, _agentBaseUrl);
                return;
            }

            // Prepare shutdown request payload
            // Format must exactly match what Agent expects
            var payload = new ShutdownRequest
            {
                Reason = reason,
                At = DateTimeOffset.Now.ToUnixTimeSeconds()
            };

            var json = JsonConvert.SerializeObject(payload);
            var content = new StringContent(json, Encoding.UTF8, "application/json");
            
            // Construct callback URL - must match Agent's shutdown endpoint exactly
            var url = $"{_agentBaseUrl}/rooms/{_roomId}/shutdown";

            _logger.LogInformation("Sending shutdown callback: {Url} reason={Reason}", url, reason);
            Console.WriteLine($"Sending shutdown callback: {url} reason={Reason}");

            try
            {
                // Prepare HTTP request with proper authentication
                // Agent validates Bearer token for security
                var request = new HttpRequestMessage(HttpMethod.Post, url)
                {
                    Content = content
                };
                request.Headers.Authorization = new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", _bearerToken);

                // Implement retry logic for reliability
                // Agent may be temporarily unavailable during shutdown
                HttpResponseMessage? response = null;
                for (int attempt = 0; attempt < 2; attempt++)
                {
                    try
                    {
                        response = await _httpClient.SendAsync(request);
                        break;
                    }
                    catch (Exception ex) when (attempt == 0)
                    {
                        Console.WriteLine($"Retry sending shutdown callback after error: {ex.Message}");
                        await Task.Delay(500);
                    }
                }
                
                if (response == null)
                {
                    Console.WriteLine("Failed to create HTTP response for shutdown callback");
                    return;
                }
                
                var responseContent = await response.Content.ReadAsStringAsync();

                if (response.IsSuccessStatusCode)
                {
                    _logger.LogInformation("Shutdown callback sent successfully: status={StatusCode}", response.StatusCode);
                    Console.WriteLine($"Shutdown callback sent OK: status={(int)response.StatusCode}");
                }
                else
                {
                    _logger.LogError("Shutdown callback failed with status: {StatusCode}, content: {Content}", 
                        response.StatusCode, responseContent);
                    Console.WriteLine($"Shutdown callback failed: status={(int)response.StatusCode} content={responseContent}");
                }
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Shutdown callback failed");
                Console.WriteLine($"Shutdown callback exception: {ex.Message}");
            }
        }

        /// <summary>
        /// Starts monitoring and graceful shutdown logic
        /// Implements the exact shutdown conditions expected by Agent
        /// </summary>
        public async Task StartMonitoringAsync()
        {
            _logger.LogInformation("Starting monitoring with initial grace: {InitialGrace}", _initialGrace);
            
            // Wait for initial grace period to allow server to fully start
            // This prevents premature shutdown during startup
            await Task.Delay(_initialGrace);

            // Check if no players connected after grace period
            // Agent expects "no_clients" reason for this scenario
            if (_players.Size == 0)
            {
                _logger.LogInformation("No players within {InitialGrace}; shutting down", _initialGrace);
                await SendShutdownCallbackAsync("no_clients");
                _shutdownCts.Cancel();
                return;
            }

            // Monitor client disconnect continuously
            // Agent expects "client_disconnected" reason for this scenario
            while (!_shutdownCts.Token.IsCancellationRequested)
            {
                await Task.Delay(TimeSpan.FromSeconds(1), _shutdownCts.Token);

                var (hasDisconnected, playerId) = _players.AnyDisconnected(_heartbeatTtl);
                if (hasDisconnected)
                {
                    _logger.LogInformation("Player {PlayerId} disconnected; shutting down", playerId);
                    await SendShutdownCallbackAsync("client_disconnected");
                    _shutdownCts.Cancel();
                    return;
                }
            }
        }

        /// <summary>
        /// Handles signal-based shutdown (SIGINT/SIGTERM)
        /// Agent expects "signal_received" reason for this scenario
        /// </summary>
        public async Task HandleSignalShutdownAsync()
        {
            _logger.LogInformation("Received signal; sending graceful shutdown");
            Console.WriteLine("Received signal; sending graceful shutdown");
            await SendShutdownCallbackAsync("signal_received");
        }

        /// <summary>
        /// Gets current player list snapshot
        /// Used by /players endpoint to provide real-time status
        /// </summary>
        public List<PlayerInfo> GetPlayers()
        {
            return _players.Snapshot(_heartbeatTtl);
        }

        /// <summary>
        /// Records heartbeat from a player
        /// Updates connection tracking for monitoring
        /// </summary>
        public void Heartbeat(string playerId)
        {
            _players.Heartbeat(playerId);
        }

        /// <summary>
        /// Cancellation token for shutdown coordination
        /// Used by other components to respond to shutdown signals
        /// </summary>
        public CancellationToken ShutdownToken => _shutdownCts.Token;

        /// <summary>
        /// Room identifier for this game server instance
        /// Must match the room_id passed by Agent
        /// </summary>
        public string RoomId => _roomId;
    }

    /// <summary>
    /// API controller for game server endpoints
    /// Must implement the exact endpoints expected by Agent and clients
    /// Note: No [controller] route prefix - endpoints must be direct as Agent expects
    /// </summary>
    [ApiController]
    public class GameController : ControllerBase
    {
        private readonly GameServer _gameServer;
        private readonly ILogger<GameController> _logger;

        public GameController(GameServer gameServer, ILogger<GameController> logger)
        {
            _gameServer = gameServer;
            _logger = logger;
        }

        /// <summary>
        /// Heartbeat endpoint - records player activity
        /// Agent uses this for readiness/liveness checks
        /// Endpoint: GET /heartbeat?player_id=<player_id>
        /// </summary>
        /// <param name="playerId">Player identifier from query parameter</param>
        /// <returns>Success confirmation</returns>
        [HttpGet("heartbeat")]
        public IActionResult Heartbeat([FromQuery] string playerId)
        {
            if (string.IsNullOrEmpty(playerId))
            {
                return BadRequest(new { error = "player_id is required" });
            }

            _gameServer.Heartbeat(playerId);
            return Ok(new { ok = true });
        }

        /// <summary>
        /// Players endpoint - returns current player list and room information
        /// Agent polls this to monitor game server status
        /// Endpoint: GET /players
        /// Response format must exactly match Go server for consistency
        /// </summary>
        /// <returns>Player list and room information</returns>
        [HttpGet("players")]
        public IActionResult Players()
        {
            var players = _gameServer.GetPlayers();
            return Ok(new { players, room_id = _gameServer.RoomId });
        }

        /// <summary>
        /// Root endpoint - provides basic server information
        /// Agent uses this for readiness checks and UI display
        /// Endpoint: GET /
        /// </summary>
        /// <returns>Server status and basic information</returns>
        [HttpGet("/")]
        public IActionResult Root()
        {
            var players = _gameServer.GetPlayers();
            var connectedCount = players.Count(p => p.State == "connected");
            var disconnectedCount = players.Count(p => p.State == "disconnected");
            
            return Ok(new { 
                room_id = _gameServer.RoomId,
                connected_players = connectedCount,
                disconnected_players = disconnectedCount,
                total_players = players.Count,
                status = "running"
            });
        }

        /// <summary>
        /// Trigger signal shutdown endpoint - for testing signal shutdown
        /// Not part of production API - used for development/testing
        /// </summary>
        [HttpPost("trigger-shutdown")]
        public async Task<IActionResult> TriggerShutdown()
        {
            _logger.LogInformation("Triggering signal shutdown via HTTP endpoint");
            await _gameServer.HandleSignalShutdownAsync();
            return Ok(new { ok = true, message = "Signal shutdown triggered" });
        }
    }

    /// <summary>
    /// Main program entry point
    /// Implements the exact command line interface expected by Agent
    /// Must parse arguments in the format: -serverPort <port> -serverId <room_id> -token <bearer_token> -agentUrl <agent_url> [-nographics] [-batchmode]
    /// </summary>
    public class Program
    {
        public static async Task Main(string[] args)
        {
            // Parse command line arguments using the new flag-based format
            // This matches exactly what the Agent expects when launching game servers
            string serverPort = string.Empty;
            string roomId = string.Empty;
            string bearerToken = string.Empty;
            string agentUrl = string.Empty;

            // Parse arguments in the format: -serverPort <port> -serverId <room_id> -token <bearer_token> -agentUrl <agent_url> [-nographics] [-batchmode]
            // This is the standard format used by the Hive Agent
            for (int i = 0; i < args.Length; i++)
            {
                switch (args[i])
                {
                    case "-serverPort":
                        if (i + 1 < args.Length) { serverPort = args[i + 1]; i++; }
                        break;
                    case "-serverId":
                        if (i + 1 < args.Length) { roomId = args[i + 1]; i++; }
                        break;
                    case "-token":
                        if (i + 1 < args.Length) { bearerToken = args[i + 1]; i++; }
                        break;
                    case "-agentUrl":
                        if (i + 1 < args.Length) { agentUrl = args[i + 1]; i++; }
                        break;
                }
            }

            // Validate required arguments
            // Agent must provide serverPort, roomId, and agentUrl for proper operation
            if (string.IsNullOrWhiteSpace(serverPort) || string.IsNullOrWhiteSpace(roomId) || string.IsNullOrWhiteSpace(agentUrl))
            {
                Console.WriteLine("Usage: cs_server -serverPort <port> -serverId <room_id> -token <bearer_token> -agentUrl <agent_url> [-nographics] [-batchmode]");
                Console.WriteLine("Example: cs_server -serverPort 8080 -serverId abc123 -token 1234abcd -agentUrl http://localhost:8080");
                Console.WriteLine("Note: -token, -nographics, -batchmode are optional");
                return;
            }
            
            // Set default bearer token if not provided
            // This matches the default used by the Go server
            if (string.IsNullOrWhiteSpace(bearerToken))
            {
                bearerToken = "1234abcd";
            }
            
            // Validate serverPort
            if (!int.TryParse(serverPort, out _))
            {
                Console.WriteLine("Error: Invalid serverPort value: {0}", serverPort);
                return;
            }
            
            // Get Agent base URL from arguments (no longer from environment variable)
            var agentBaseUrl = agentUrl;

            Console.WriteLine($"Starting CS Game Server:");
            Console.WriteLine($"  Server Port: {serverPort}");
            Console.WriteLine($"  Room ID: {roomId}");
            Console.WriteLine($"  Bearer Token: {(bearerToken.Length > 4 ? bearerToken[..4] + "..." : bearerToken)}");
            Console.WriteLine($"  Agent URL: {agentBaseUrl}");
            Console.WriteLine($"  Protocol: Hive Agent v2 (flag-based arguments)");

            // Create web application builder
            var builder = WebApplication.CreateBuilder(args);

            // Configure services
            builder.Services.AddControllers();
            builder.Services.AddLogging(logging =>
            {
                logging.AddConsole();
                logging.SetMinimumLevel(LogLevel.Information);
            });

            // Configure CORS for client access
            // Must allow all origins as specified in Agent documentation
            builder.Services.AddCors(options =>
            {
                options.AddDefaultPolicy(policy =>
                {
                    policy.AllowAnyOrigin()
                          .AllowAnyMethod()
                          .AllowAnyHeader()
                          .WithExposedHeaders("Content-Type");
                });
            });

            // Register GameServer as singleton service
            // This ensures consistent state across all requests
            builder.Services.AddSingleton<GameServer>(provider =>
            {
                var gameLogger = provider.GetRequiredService<ILogger<GameServer>>();
                var playerLogger = provider.GetRequiredService<ILogger<PlayerStore>>();
                return new GameServer(gameLogger, playerLogger, roomId, bearerToken, agentBaseUrl);
            });

            var app = builder.Build();

            // Configure middleware
            app.UseCors();
            app.UseRouting();
            app.MapControllers();

            // Configure port binding
            // Must bind to 0.0.0.0 to accept connections from Agent
            app.Urls.Clear();
            app.Urls.Add($"http://0.0.0.0:{serverPort}");

            // Start monitoring in background
            // This implements the graceful shutdown logic expected by Agent
            var gameServer = app.Services.GetRequiredService<GameServer>();
            _ = Task.Run(() => gameServer.StartMonitoringAsync());

            // Handle signal shutdown (SIGINT/SIGTERM)
            // Agent expects proper signal handling for graceful shutdown
            var cts = new CancellationTokenSource();
            Console.CancelKeyPress += async (sender, e) =>
            {
                e.Cancel = true;
                Console.WriteLine("Received Ctrl+C, initiating graceful shutdown...");
                await gameServer.HandleSignalShutdownAsync();
                // Wait briefly to ensure callback reaches Agent before process termination
                await Task.Delay(1000);
                cts.Cancel();
            };

            // Handle SIGTERM (docker stop, kill -15) via Unloading
            // This is critical for containerized deployments
            AssemblyLoadContext.Default.Unloading += _ =>
            {
                try
                {
                    Console.WriteLine("SIGTERM received, initiating graceful shutdown...");
                    gameServer.HandleSignalShutdownAsync().GetAwaiter().GetResult();
                    // Wait briefly to ensure callback reaches Agent before process termination
                    Thread.Sleep(1000);
                    cts.Cancel();
                }
                catch { /* best-effort shutdown */ }
            };

            // Best-effort callback on ProcessExit
            // Note: This won't work with SIGKILL (-9) as expected
            AppDomain.CurrentDomain.ProcessExit += (sender, e) =>
            {
                try
                {
                    Console.WriteLine("ProcessExit, sending graceful shutdown callback...");
                    gameServer.HandleSignalShutdownAsync().GetAwaiter().GetResult();
                    // Best-effort wait for callback completion
                    Thread.Sleep(500);
                }
                catch { /* best-effort shutdown */ }
            };

            try
            {
                Console.WriteLine($"Server listening on :{serverPort} room={roomId}");
                Console.WriteLine("Ready to accept connections from Agent and clients");
                await app.RunAsync(cts.Token);
            }
            catch (OperationCanceledException)
            {
                Console.WriteLine("Server shutdown requested");
            }
            catch (Exception ex)
            {
                Console.WriteLine($"Server error: {ex.Message}");
            }
            finally
            {
                Console.WriteLine("Game server shutdown complete");
            }
        }
    }
}
