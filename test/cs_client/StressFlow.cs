using System;
using System.Collections.Concurrent;
using System.Net.Http;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;
using System.Security.Cryptography;
using System.Threading;
using System.Threading.Tasks;

namespace CsClient
{
	public static class StressFlow
	{
		public const string AgentBaseUrl = "http://52.221.213.97:8080";
		private const int TICKET_POLL_DELAY_SECONDS = 5;
		private const int TICKET_MAX_WAIT_SECONDS = 150;
		private const int ROOM_POLL_DELAY_SECONDS = 3;
		private const int ROOM_MAX_WAIT_SECONDS = 180;
		private const int HEARTBEAT_DELAY_SECONDS = 5;
		private const int HEARTBEAT_TOTAL_SECONDS = 60;
		private const int MAX_ACTIVE_CLIENTS = 20;

		private static readonly HttpClient Http = new HttpClient { Timeout = TimeSpan.FromSeconds(10) };
		private static readonly ConcurrentDictionary<string, RoomRow> RoomTable = new();
		private static int ActiveClients = 0;
		private static readonly object RenderLock = new();

		public static async Task Run()
		{
			Console.OutputEncoding = Encoding.UTF8;
			Console.WriteLine("🧪 Stress flow starting... (spawn client every 5s, up to 100 active)\n");

			var renderCts = new CancellationTokenSource();
			var renderTask = Task.Run(() => RenderLoop(renderCts.Token));

			while (true)
			{
				// Gate by active clients
				if (ActiveClients >= MAX_ACTIVE_CLIENTS)
				{
					await Task.Delay(1000);
					continue;
				}

				// Fixed delay 1s before spawning next client (increased intensity)
				await Task.Delay(1000);

				_ = Task.Run(async () => await RunSingleClient());
			}
		}

		private static async Task RunSingleClient()
		{
			Interlocked.Increment(ref ActiveClients);
			try
			{
				var playerId = GenerateRandomPlayerId();
				string ticketId = await SubmitTicket(playerId);
				string roomId = await PollTicketUntilMatched(ticketId);

				// Ensure row exists for this room
				var row = RoomTable.GetOrAdd(roomId, _ => new RoomRow { RoomId = roomId });
				row.Status = "OPENED";
				row.AddPlayer(playerId);

				var (ip, port) = await PollRoomUntilReady(roomId, row);
				row.ServerUrl = $"http://{ip}:{port}";
				row.Status = "ACTIVED";

				// Heartbeat for 60s, then stop and remove room from table
				await HeartbeatWindow(row.ServerUrl, playerId, HEARTBEAT_TOTAL_SECONDS, row);
			}
			catch (Exception)
			{
				// Minimal log, keep console clean under stress
			}
			finally
			{
				Interlocked.Decrement(ref ActiveClients);
			}
		}

		private static async Task HeartbeatWindow(string serverBaseUrl, string playerId, int totalSeconds, RoomRow row)
		{
			int loops = totalSeconds / HEARTBEAT_DELAY_SECONDS;
			for (int i = 0; i < loops; i++)
			{
				var url = $"{serverBaseUrl}/heartbeat?player_id={Uri.EscapeDataString(playerId)}";
				try
				{
					var resp = await Http.GetAsync(url);
					row.Heartbeat = resp.IsSuccessStatusCode ? "OK" : $"HTTP {(int)resp.StatusCode}";
				}
				catch
				{
					row.Heartbeat = "ERR";
				}
				await Task.Delay(TimeSpan.FromSeconds(HEARTBEAT_DELAY_SECONDS));
			}
			row.Heartbeat = "STOPPED";
			// Remove room from table as soon as any player's heartbeat ends
			RoomTable.TryRemove(row.RoomId, out _);
		}

		private static async Task<string> SubmitTicket(string playerId)
		{
			var url = $"{AgentBaseUrl}/tickets";
			var body = JsonSerializer.Serialize(new { player_id = playerId });
			var resp = await Http.PostAsync(url, new StringContent(body, Encoding.UTF8, "application/json"));
			var raw = await resp.Content.ReadAsStringAsync();
			if (!resp.IsSuccessStatusCode) throw new Exception($"submit ticket failed: {(int)resp.StatusCode}");
			var obj = SafeDeserialize<SubmitTicketResponse>(raw) ?? throw new Exception("decode ticket resp");
			if (!string.IsNullOrEmpty(obj.Status) && obj.Status.Equals("REJECTED", StringComparison.OrdinalIgnoreCase)) throw new Exception("ticket rejected");
			return obj.TicketId ?? throw new Exception("missing ticket_id");
		}

		private static async Task<string> PollTicketUntilMatched(string ticketId)
		{
			int loops = TICKET_MAX_WAIT_SECONDS / TICKET_POLL_DELAY_SECONDS;
			for (int i = 0; i < loops; i++)
			{
				var url = $"{AgentBaseUrl}/tickets/{ticketId}";
				HttpResponseMessage resp; string raw;
				try { resp = await Http.GetAsync(url); raw = await resp.Content.ReadAsStringAsync(); }
				catch { await Task.Delay(TimeSpan.FromSeconds(TICKET_POLL_DELAY_SECONDS)); continue; }
				if (!resp.IsSuccessStatusCode) { await Task.Delay(TimeSpan.FromSeconds(TICKET_POLL_DELAY_SECONDS)); continue; }
				var obj = SafeDeserialize<TicketStatusResponse>(raw);
				if (obj?.Status?.Equals("MATCHED", StringComparison.OrdinalIgnoreCase) == true && !string.IsNullOrEmpty(obj.RoomId)) return obj.RoomId!;
				if (obj?.Status?.Equals("EXPIRED", StringComparison.OrdinalIgnoreCase) == true || obj?.Status?.Equals("REJECTED", StringComparison.OrdinalIgnoreCase) == true) throw new Exception("ticket fail");
				await Task.Delay(TimeSpan.FromSeconds(TICKET_POLL_DELAY_SECONDS));
			}
			throw new Exception("ticket timeout");
		}

		private static async Task<(string ip, int port)> PollRoomUntilReady(string roomId, RoomRow row)
		{
			int loops = ROOM_MAX_WAIT_SECONDS / ROOM_POLL_DELAY_SECONDS;
			for (int i = 0; i < loops; i++)
			{
				var url = $"{AgentBaseUrl}/rooms/{roomId}";
				HttpResponseMessage resp; string raw;
				try { resp = await Http.GetAsync(url); raw = await resp.Content.ReadAsStringAsync(); }
				catch { await Task.Delay(TimeSpan.FromSeconds(ROOM_POLL_DELAY_SECONDS)); continue; }
				if (!resp.IsSuccessStatusCode) { await Task.Delay(TimeSpan.FromSeconds(ROOM_POLL_DELAY_SECONDS)); continue; }
				var room = SafeDeserialize<RoomState>(raw); if (room == null) { await Task.Delay(TimeSpan.FromSeconds(ROOM_POLL_DELAY_SECONDS)); continue; }
				row.Status = room.Status ?? row.Status;
				row.Players = room.Players != null ? string.Join("|", room.Players) : row.Players;
				// terminal
				if (!string.IsNullOrEmpty(room.Status) && room.Status.Equals("DEAD", StringComparison.OrdinalIgnoreCase)) throw new Exception("room DEAD");
				if (!string.IsNullOrEmpty(room.Status) && room.Status.Equals("FULFILLED", StringComparison.OrdinalIgnoreCase)) throw new Exception("room FULFILLED");
				// ready via ACTIVED
				if (!string.IsNullOrEmpty(room.Status) && room.Status.Equals("ACTIVED", StringComparison.OrdinalIgnoreCase))
				{
					var epA = ExtractEndpoint(room);
					if (!string.IsNullOrEmpty(epA.ip) && epA.port > 0) return epA;
				}
				// fallback via Nomad
				var ep = ExtractEndpoint(room);
				if (!string.IsNullOrEmpty(ep.ip) && ep.port > 0) return ep;
				await Task.Delay(TimeSpan.FromSeconds(ROOM_POLL_DELAY_SECONDS));
			}
			throw new Exception("room timeout");
		}

		private static (string ip, int port) ExtractEndpoint(RoomState room)
		{
			if (!string.IsNullOrEmpty(room.ServerIP) && room.Port.HasValue && room.Port.Value > 0) return (room.ServerIP!, room.Port!.Value);
			if (!string.IsNullOrEmpty(room.HostIP))
			{
				int port = 0;
				if (room.Ports != null)
				{
					if (room.Ports.TryGetValue("http", out var httpPort)) port = httpPort;
					else if (room.Ports.Count > 0) foreach (var p in room.Ports.Values) { port = p; break; }
				}
				if (port > 0) return (room.HostIP!, port);
			}
			return (string.Empty, 0);
		}

		private static void RenderLoop(CancellationToken token)
		{
			while (!token.IsCancellationRequested)
			{
				lock (RenderLock)
				{
					Console.Clear();
					Console.WriteLine($"🧪 Stress Flow | Active clients: {ActiveClients} | Rooms: {RoomTable.Count}");
					Console.WriteLine("room_id                                | status     | players                      | server url                     | hb");
					Console.WriteLine(new string('-', 120));
					foreach (var kv in RoomTable)
					{
						var r = kv.Value;
						Console.WriteLine($"{Trunc(r.RoomId,36),-36} | {Trunc(r.Status,10),-10} | {Trunc(r.Players,28),-28} | {Trunc(r.ServerUrl,28),-28} | {Trunc(r.Heartbeat,6),-6}");
					}
				}
				Thread.Sleep(1000);
			}
		}

		private static string Trunc(string? s, int n) => (s ?? string.Empty).Length <= n ? (s ?? string.Empty) : (s!.Substring(0, n - 3) + "...");

		private static string GenerateRandomPlayerId()
		{
			string[] names = new[] { "alex", "sam", "mike", "ava", "jack", "lily", "noah", "olivia", "leo", "mila", "kai", "nina" };
			int ni = RandomNumberGenerator.GetInt32(0, names.Length);
			int digits = RandomNumberGenerator.GetInt32(0, 1000);
			return $"{names[ni]}{digits:D3}";
		}

		// Models + helpers
		private class SubmitTicketResponse { [JsonPropertyName("ticket_id")] public string? TicketId { get; set; } [JsonPropertyName("status")] public string? Status { get; set; } }
		private class TicketStatusResponse { [JsonPropertyName("status")] public string? Status { get; set; } [JsonPropertyName("room_id")] public string? RoomId { get; set; } }
		private class RoomState
		{
			[JsonPropertyName("room_id")] public string? RoomId { get; set; }
			[JsonPropertyName("allocation_id")] public string? AllocationId { get; set; }
			[JsonPropertyName("server_ip")] public string? ServerIP { get; set; }
			[JsonPropertyName("port")] public int? Port { get; set; }
			[JsonPropertyName("players")] public string[]? Players { get; set; }
			[JsonPropertyName("created_at_unix")] public long? CreatedAtUnix { get; set; }
			[JsonPropertyName("status")] public string? Status { get; set; }
			[JsonPropertyName("node_id")] public string? NodeId { get; set; }
			[JsonPropertyName("host_ip")] public string? HostIP { get; set; }
			[JsonPropertyName("ports")] public System.Collections.Generic.Dictionary<string, int>? Ports { get; set; }
		}

		private class RoomRow
		{
			public string RoomId { get; set; } = string.Empty;
			public string Status { get; set; } = string.Empty;
			public string Players { get; set; } = string.Empty;
			public string ServerUrl { get; set; } = string.Empty;
			public string Heartbeat { get; set; } = string.Empty;
			public void AddPlayer(string p) { Players = string.IsNullOrEmpty(Players) ? p : (Players + "|" + p); }
		}

		private static T? SafeDeserialize<T>(string raw)
		{ try { return JsonSerializer.Deserialize<T>(raw, new JsonSerializerOptions { PropertyNameCaseInsensitive = true }); } catch { return default; } }
		private static int GetRandomInt(int minInclusive, int maxExclusive) => RandomNumberGenerator.GetInt32(minInclusive, maxExclusive);
	}
}
