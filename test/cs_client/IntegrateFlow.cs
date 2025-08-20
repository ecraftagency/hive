using System;
using System.Net.Http;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;
using System.Security.Cryptography;
using System.Threading.Tasks;

/*
================================================================================
Integration Flow (Agent v1) – Hướng dẫn tích hợp C#
================================================================================
Mục tiêu: Mẫu code tối giản, có thể copy/paste để tích hợp Web/Game client với Agent v1.

Luồng tổng quát:
1) Gọi POST /tickets với player_id (random name + 3 số)
2) Poll GET /tickets/:ticket_id mỗi 5s đến khi status=MATCHED → nhận room_id
3) Poll GET /rooms/:room_id mỗi 3s đến khi status=FULFILLED (hoặc có host_ip+port) → nhận endpoint http://IP:PORT
4) Gửi heartbeat trực tiếp đến http://IP:PORT/heartbeat?player_id=...

Lưu ý quan trọng:
- Poll ticket tối đa 150s (>= TTL 120s), ticket có thể EXPIRED → dừng sớm.
- Poll room tối đa 180s, nếu DEAD → dừng và báo lỗi; nếu có host_ip+port (Nomad) coi như fulfilled.
- Heartbeat không qua proxy của agent trong integrate flow (gửi trực tiếp tới server).
- Bật JSON PropertyNameCaseInsensitive để khớp casing linh hoạt.
- Log lỗi HTTP (status code + body) để dễ điều tra.
================================================================================
*/

namespace CsClient
{
	public static class IntegrateFlow
	{
		public const string AgentBaseUrl = "http://52.221.213.97:8080";
		private const int TICKET_POLL_DELAY_SECONDS = 5;
		private const int TICKET_MAX_WAIT_SECONDS = 150;
		private const int ROOM_POLL_DELAY_SECONDS = 3;
		private const int ROOM_MAX_WAIT_SECONDS = 180;
		private const int HEARTBEAT_DELAY_SECONDS = 5;

		// Entry: chạy full flow cho 2 người chơi demo
		public static async Task Run()
		{
			using var http = new HttpClient { Timeout = TimeSpan.FromSeconds(10) };
			var player1 = GenerateRandomPlayerId();
			var player2 = GenerateRandomPlayerId();
			if (player2 == player1) player2 = GenerateRandomPlayerId();
			Console.WriteLine($"👤 Players: player1={player1}, player2={player2}\n");

			// 1) Submit ticket
			var t1 = SubmitTicket(http, player1);
			var t2 = SubmitTicket(http, player2);
			await Task.WhenAll(t1, t2);
			var ticket1 = t1.Result; var ticket2 = t2.Result;
			Console.WriteLine($"🎫 Ticket1: {ticket1}");
			Console.WriteLine($"🎫 Ticket2: {ticket2}\n");

			// 2) Poll ticket → MATCHED
			var roomId1 = await PollTicketUntilMatched(http, ticket1);
			var roomId2 = await PollTicketUntilMatched(http, ticket2);
			Console.WriteLine($"🎯 Matched: room1={roomId1}, room2={roomId2}\n");

			// 3) Poll room → FULFILLED (hoặc có host_ip+port)
			var ep1Task = PollRoomUntilFulfilled(http, roomId1);
			var ep2Task = PollRoomUntilFulfilled(http, roomId2);
			var ep1 = await ep1Task; var ep2 = await ep2Task;
			var url1 = $"http://{ep1.ip}:{ep1.port}";
			var url2 = $"http://{ep2.ip}:{ep2.port}";
			Console.WriteLine($"🏠 Room1 fulfilled endpoint: {ep1.ip}:{ep1.port} → {url1}");
			Console.WriteLine($"🏠 Room2 fulfilled endpoint: {ep2.ip}:{ep2.port} → {url2}\n");

			// 4) Heartbeat trực tiếp tới server (vô hạn)
			await Task.WhenAll(
				HeartbeatLoopDirect(http, url1, player1),
				HeartbeatLoopDirect(http, url2, player2)
			);
		}

		// Sinh player_id ngẫu nhiên theo name + 3 số (VD: alex123)
		private static string GenerateRandomPlayerId()
		{
			string[] names = new[] { "alex", "sam", "mike", "ava", "jack", "lily", "noah", "olivia", "leo", "mila", "kai", "nina" };
			int ni = RandomNumberGenerator.GetInt32(0, names.Length);
			int digits = RandomNumberGenerator.GetInt32(0, 1000);
			return $"{names[ni]}{digits:D3}";
		}

		// POST /tickets → trả ticket_id
		private static async Task<string> SubmitTicket(HttpClient http, string playerId)
		{
			var url = $"{AgentBaseUrl}/tickets";
			var body = JsonSerializer.Serialize(new { player_id = playerId });
			var resp = await http.PostAsync(url, new StringContent(body, Encoding.UTF8, "application/json"));
			var raw = await resp.Content.ReadAsStringAsync();
			if (!resp.IsSuccessStatusCode) throw new Exception($"submit ticket failed: HTTP {(int)resp.StatusCode}, body={raw}");
			var obj = SafeDeserialize<SubmitTicketResponse>(raw);
			if (obj == null || string.IsNullOrEmpty(obj.TicketId)) throw new Exception($"submit ticket decode failed: body={raw}");
			if (!string.IsNullOrEmpty(obj.Status) && obj.Status.Equals("REJECTED", StringComparison.OrdinalIgnoreCase)) throw new Exception("ticket rejected by agent");
			return obj.TicketId!;
		}

		// Poll GET /tickets/:id → MATCHED => trả room_id; EXPIRED/REJECTED → lỗi; timeout 150s
		private static async Task<string> PollTicketUntilMatched(HttpClient http, string ticketId)
		{
			Console.WriteLine($"🔍 Polling ticket {ticketId}...");
			int loops = TICKET_MAX_WAIT_SECONDS / TICKET_POLL_DELAY_SECONDS;
			for (int i = 0; i < loops; i++)
			{
				var url = $"{AgentBaseUrl}/tickets/{ticketId}";
				HttpResponseMessage resp; string raw;
				try { resp = await http.GetAsync(url); raw = await resp.Content.ReadAsStringAsync(); }
				catch (Exception ex) { Console.WriteLine($"   poll error: {ex.Message}"); await Task.Delay(TimeSpan.FromSeconds(TICKET_POLL_DELAY_SECONDS)); continue; }
				if (!resp.IsSuccessStatusCode) { Console.WriteLine($"   poll HTTP {(int)resp.StatusCode}: {raw}"); await Task.Delay(TimeSpan.FromSeconds(TICKET_POLL_DELAY_SECONDS)); continue; }
				var obj = SafeDeserialize<TicketStatusResponse>(raw);
				var status = obj?.Status ?? string.Empty; var rid = obj?.RoomId ?? string.Empty;
				Console.WriteLine($"   status={status} room_id={rid}");
				if (status.Equals("MATCHED", StringComparison.OrdinalIgnoreCase) && !string.IsNullOrEmpty(rid)) return rid;
				if (status.Equals("EXPIRED", StringComparison.OrdinalIgnoreCase)) throw new Exception("ticket expired (TTL exceeded)");
				if (status.Equals("REJECTED", StringComparison.OrdinalIgnoreCase)) throw new Exception("ticket rejected by agent");
				await Task.Delay(TimeSpan.FromSeconds(TICKET_POLL_DELAY_SECONDS));
			}
			throw new Exception("poll ticket timeout");
		}

		// Poll GET /rooms/:room_id → FULFILLED (hoặc có host_ip+port) => trả endpoint http://IP:PORT; DEAD => lỗi; timeout 180s
		private static async Task<(string ip, int port)> PollRoomUntilFulfilled(HttpClient http, string roomId)
		{
			Console.WriteLine($"🔎 Polling room {roomId} until FULFILLED...");
			int loops = ROOM_MAX_WAIT_SECONDS / ROOM_POLL_DELAY_SECONDS;
			for (int i = 0; i < loops; i++)
			{
				var url = $"{AgentBaseUrl}/rooms/{roomId}";
				HttpResponseMessage resp; string raw;
				try { resp = await http.GetAsync(url); raw = await resp.Content.ReadAsStringAsync(); }
				catch (Exception ex) { Console.WriteLine($"   room poll error: {ex.Message}"); await Task.Delay(TimeSpan.FromSeconds(ROOM_POLL_DELAY_SECONDS)); continue; }
				if (!resp.IsSuccessStatusCode) { Console.WriteLine($"   room HTTP {(int)resp.StatusCode}: {raw}"); await Task.Delay(TimeSpan.FromSeconds(ROOM_POLL_DELAY_SECONDS)); continue; }
				var room = SafeDeserialize<RoomState>(raw); if (room == null) { Console.WriteLine("   room decode failed"); await Task.Delay(TimeSpan.FromSeconds(ROOM_POLL_DELAY_SECONDS)); continue; }
				var status = room.Status ?? string.Empty;
				if (!string.IsNullOrEmpty(status))
				{
					if (status.Equals("DEAD", StringComparison.OrdinalIgnoreCase)) throw new Exception("room DEAD (allocation failed)");
					if (status.Equals("FULFILLED", StringComparison.OrdinalIgnoreCase)) { var ep = ExtractEndpoint(room); if (!string.IsNullOrEmpty(ep.ip) && ep.port > 0) return ep; }
				}
				else { var ep = ExtractEndpoint(room); if (!string.IsNullOrEmpty(ep.ip) && ep.port > 0) return ep; }
				await Task.Delay(TimeSpan.FromSeconds(ROOM_POLL_DELAY_SECONDS));
			}
			throw new Exception("poll room timeout");
		}

		// Chọn endpoint: ưu tiên server_ip/port (Redis), nếu không có thì host_ip/ports["http"] (Nomad)
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

		// Heartbeat trực tiếp: GET http://IP:PORT/heartbeat?player_id=...
		private static async Task HeartbeatLoopDirect(HttpClient http, string serverBaseUrl, string playerId)
		{
			Console.WriteLine($"❤️ Starting direct heartbeat: server={serverBaseUrl} player={playerId}");
			int i = 0;
			while (true)
			{
				var url = $"{serverBaseUrl}/heartbeat?player_id={Uri.EscapeDataString(playerId)}";
				try { var resp = await http.GetAsync(url); var raw = await resp.Content.ReadAsStringAsync(); Console.WriteLine($"   HB[{i}] status={(int)resp.StatusCode} body={raw}"); }
				catch (Exception ex) { Console.WriteLine($"   HB error: {ex.Message}"); }
				i++; await Task.Delay(TimeSpan.FromSeconds(HEARTBEAT_DELAY_SECONDS));
			}
		}

		// JSON models
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

		private static T? SafeDeserialize<T>(string raw)
		{
			try { return JsonSerializer.Deserialize<T>(raw, new JsonSerializerOptions { PropertyNameCaseInsensitive = true }); }
			catch { return default; }
		}
	}
}

