using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Mvc;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using Newtonsoft.Json;
using System.Collections.Concurrent;
using System.Net;
using System.Text;

namespace CsServer
{
    /// <summary>
    /// Thông tin người chơi với trạng thái kết nối
    /// </summary>
    public class PlayerInfo
    {
        public string PlayerId { get; set; } = string.Empty;
        public string State { get; set; } = "connected";
        public long LastSeenUnix { get; set; }
    }

    /// <summary>
    /// Request body cho shutdown callback đến Agent
    /// </summary>
    public class ShutdownRequest
    {
        public string Reason { get; set; } = string.Empty;
        public long At { get; set; }
    }

    /// <summary>
    /// Response cho shutdown callback
    /// </summary>
    public class ShutdownResponse
    {
        public bool Ok { get; set; }
        public string Status { get; set; } = string.Empty;
    }

    /// <summary>
    /// Quản lý danh sách người chơi và heartbeat
    /// </summary>
    public class PlayerStore
    {
        private readonly ConcurrentDictionary<string, DateTime> _lastSeen = new();
        private readonly ILogger<PlayerStore> _logger;

        public PlayerStore(ILogger<PlayerStore> logger)
        {
            _logger = logger;
        }

        /// <summary>
        /// Ghi nhận heartbeat từ người chơi
        /// </summary>
        public void Heartbeat(string playerId)
        {
            _lastSeen.AddOrUpdate(playerId, DateTime.Now, (_, _) => DateTime.Now);
            _logger.LogInformation("Heartbeat from {PlayerId}", playerId);
        }

        /// <summary>
        /// Lấy số lượng người chơi hiện tại
        /// </summary>
        public int Size => _lastSeen.Count;

        /// <summary>
        /// Tạo snapshot danh sách người chơi với trạng thái
        /// </summary>
        public List<PlayerInfo> Snapshot(TimeSpan ttl)
        {
            var now = DateTime.Now;
            var result = new List<PlayerInfo>();

            foreach (var kvp in _lastSeen)
            {
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
        /// Kiểm tra có người chơi nào bị disconnect không
        /// </summary>
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
    /// Game Server chính - tương tự cmd/server/main.go
    /// </summary>
    public class GameServer
    {
        private readonly ILogger<GameServer> _logger;
        private readonly PlayerStore _players;
        private readonly string _roomId;
        private readonly string _bearerToken;
        private readonly string _agentBaseUrl;
        private readonly TimeSpan _heartbeatTtl = TimeSpan.FromSeconds(10);
        private readonly TimeSpan _initialGrace = TimeSpan.FromSeconds(20);
        private readonly CancellationTokenSource _shutdownCts = new();
        private readonly HttpClient _httpClient = new();

        public GameServer(ILogger<GameServer> logger, ILogger<PlayerStore> playerLogger, string roomId, string bearerToken, string agentBaseUrl)
        {
            _logger = logger;
            _players = new PlayerStore(playerLogger);
            _roomId = roomId;
            _bearerToken = bearerToken;
            _agentBaseUrl = agentBaseUrl;
            _httpClient.Timeout = TimeSpan.FromSeconds(5);
        }

        /// <summary>
        /// Gửi shutdown callback đến Agent
        /// </summary>
        private async Task SendShutdownCallbackAsync(string reason)
        {
            if (string.IsNullOrEmpty(_roomId) || string.IsNullOrEmpty(_bearerToken) || string.IsNullOrEmpty(_agentBaseUrl))
            {
                _logger.LogWarning("Shutdown callback skipped: roomId={RoomId}, bearer={Bearer}, agentBase={AgentBase}", 
                    _roomId, _bearerToken, _agentBaseUrl);
                return;
            }

            var payload = new ShutdownRequest
            {
                Reason = reason,
                At = DateTimeOffset.Now.ToUnixTimeSeconds()
            };

            var json = JsonConvert.SerializeObject(payload);
            var content = new StringContent(json, Encoding.UTF8, "application/json");
            var url = $"{_agentBaseUrl}/rooms/{_roomId}/shutdown";

            _logger.LogInformation("Sending shutdown callback: {Url} reason={Reason}", url, reason);

            try
            {
                var request = new HttpRequestMessage(HttpMethod.Post, url)
                {
                    Content = content
                };
                request.Headers.Authorization = new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", _bearerToken);

                var response = await _httpClient.SendAsync(request);
                var responseContent = await response.Content.ReadAsStringAsync();

                if (response.IsSuccessStatusCode)
                {
                    _logger.LogInformation("Shutdown callback sent successfully: status={StatusCode}", response.StatusCode);
                }
                else
                {
                    _logger.LogError("Shutdown callback failed with status: {StatusCode}, content: {Content}", 
                        response.StatusCode, responseContent);
                }
            }
            catch (Exception ex)
            {
                _logger.LogError(ex, "Shutdown callback failed");
            }
        }

        /// <summary>
        /// Khởi chạy monitoring và graceful shutdown logic
        /// </summary>
        public async Task StartMonitoringAsync()
        {
            _logger.LogInformation("Starting monitoring with initial grace: {InitialGrace}", _initialGrace);
            
            // Đợi initial grace period
            await Task.Delay(_initialGrace);

            // Kiểm tra nếu không có người chơi
            if (_players.Size == 0)
            {
                _logger.LogInformation("No players within {InitialGrace}; shutting down", _initialGrace);
                await SendShutdownCallbackAsync("no_clients");
                _shutdownCts.Cancel();
                return;
            }

            // Monitor client disconnect
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
        /// Xử lý signal shutdown (SIGINT/SIGTERM)
        /// </summary>
        public async Task HandleSignalShutdownAsync()
        {
            _logger.LogInformation("Received signal; sending graceful shutdown");
            await SendShutdownCallbackAsync("signal_received");
        }

        /// <summary>
        /// Lấy danh sách người chơi
        /// </summary>
        public List<PlayerInfo> GetPlayers()
        {
            return _players.Snapshot(_heartbeatTtl);
        }

        /// <summary>
        /// Ghi nhận heartbeat
        /// </summary>
        public void Heartbeat(string playerId)
        {
            _players.Heartbeat(playerId);
        }

        /// <summary>
        /// Lấy CancellationToken cho shutdown
        /// </summary>
        public CancellationToken ShutdownToken => _shutdownCts.Token;

        public string RoomId => _roomId;
    }

    /// <summary>
    /// Controller xử lý các API endpoints
    /// </summary>
    [ApiController]
    [Route("[controller]")]
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
        /// Heartbeat endpoint - ghi nhận heartbeat từ client
        /// </summary>
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
        /// Players endpoint - trả danh sách người chơi
        /// </summary>
        [HttpGet("players")]
        public IActionResult Players()
        {
            var players = _gameServer.GetPlayers();
            return Ok(new { players, room_id = _gameServer.RoomId });
        }
    }

    /// <summary>
    /// Main program - entry point
    /// </summary>
    public class Program
    {
        public static async Task Main(string[] args)
        {
            // Parse command line arguments with new flag structure
            string port = string.Empty;
            string roomId = string.Empty;
            string bearerToken = string.Empty;

            for (int i = 0; i < args.Length; i++)
            {
                switch (args[i])
                {
                    case "-port":
                        if (i + 1 < args.Length) { port = args[i + 1]; i++; }
                        break;
                    case "-serverId":
                        if (i + 1 < args.Length) { roomId = args[i + 1]; i++; }
                        break;
                    case "-token":
                        if (i + 1 < args.Length) { bearerToken = args[i + 1]; i++; }
                        break;
                }
            }

            if (string.IsNullOrWhiteSpace(port) || string.IsNullOrWhiteSpace(roomId))
            {
                Console.WriteLine("Usage: cs_server -port <port> -serverId <room_id> -token <bearer_token>");
                return;
            }
            if (string.IsNullOrWhiteSpace(bearerToken))
            {
                bearerToken = "1234abcd";
            }
            var agentBaseUrl = Environment.GetEnvironmentVariable("AGENT_BASE_URL") ?? "http://127.0.0.1:8080";

            Console.WriteLine($"Starting CS Game Server:");
            Console.WriteLine($"  Port: {port}");
            Console.WriteLine($"  Room ID: {roomId}");
            Console.WriteLine($"  Bearer Token: {(bearerToken.Length > 4 ? bearerToken[..4] + "..." : bearerToken)}");
            Console.WriteLine($"  Agent Base URL: {agentBaseUrl}");

            // Tạo builder cho web application
            var builder = WebApplication.CreateBuilder(args);

            // Cấu hình services
            builder.Services.AddControllers();
            builder.Services.AddLogging(logging =>
            {
                logging.AddConsole();
                logging.SetMinimumLevel(LogLevel.Information);
            });

            // Cấu hình CORS
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

            // Register services
            builder.Services.AddSingleton<GameServer>(provider =>
            {
                var gameLogger = provider.GetRequiredService<ILogger<GameServer>>();
                var playerLogger = provider.GetRequiredService<ILogger<PlayerStore>>();
                return new GameServer(gameLogger, playerLogger, roomId, bearerToken, agentBaseUrl);
            });

            var app = builder.Build();

            // Cấu hình middleware
            app.UseCors();
            app.UseRouting();
            app.MapControllers();

            // Cấu hình port
            app.Urls.Clear();
            app.Urls.Add($"http://0.0.0.0:{port}");

            // Khởi chạy monitoring trong background
            var gameServer = app.Services.GetRequiredService<GameServer>();
            _ = Task.Run(() => gameServer.StartMonitoringAsync());

            // Xử lý signal shutdown
            var cts = new CancellationTokenSource();
            Console.CancelKeyPress += async (sender, e) =>
            {
                e.Cancel = true;
                Console.WriteLine("Received Ctrl+C, initiating graceful shutdown...");
                await gameServer.HandleSignalShutdownAsync();
                cts.Cancel();
            };

            try
            {
                Console.WriteLine($"Server listening on :{port} room={roomId}");
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
                Console.WriteLine("Game finish");
            }
        }
    }
}
