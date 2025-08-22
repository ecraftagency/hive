using Microsoft.Extensions.Logging;
using System.Net;
using System.Text;
using System.Text.Json;

namespace IntegrationAgent;

/// <summary>
/// Request body cho shutdown callback từ Server
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
/// Xử lý shutdown callback từ Server
/// </summary>
public class ShutdownCallbackHandler
{
    private readonly ILogger<ShutdownCallbackHandler> _logger;
    private readonly TestConfig _config;
    private readonly HttpListener _httpListener;
    private readonly CancellationTokenSource _cancellationTokenSource;
    private readonly Task _listenerTask;
    
    public event Action<string, long>? OnShutdownReceived;
    
    public ShutdownCallbackHandler(ILogger<ShutdownCallbackHandler> logger, TestConfig config)
    {
        _logger = logger;
        _config = config;
        _httpListener = new HttpListener();
        _cancellationTokenSource = new CancellationTokenSource();
        
        // Lắng nghe trên cổng được cấu hình
        _httpListener.Prefixes.Add($"http://localhost:{_config.AgentPort}/");
        
        _listenerTask = Task.Run(ListenAsync);
    }
    
    private async Task ListenAsync()
    {
        try
        {
            _httpListener.Start();
            _logger.LogInformation("✅ Shutdown callback handler listening on port {Port}", _config.AgentPort);
            
            while (!_cancellationTokenSource.Token.IsCancellationRequested)
            {
                try
                {
                    var context = await _httpListener.GetContextAsync();
                    _ = Task.Run(() => HandleRequestAsync(context));
                }
                catch (HttpListenerException) when (_cancellationTokenSource.Token.IsCancellationRequested)
                {
                    break;
                }
                catch (Exception ex)
                {
                    _logger.LogError(ex, "Error handling HTTP request");
                }
            }
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error starting HTTP listener");
        }
        finally
        {
            _httpListener.Stop();
            _logger.LogInformation("Shutdown callback handler stopped");
        }
    }
    
    private async Task HandleRequestAsync(HttpListenerContext context)
    {
        var request = context.Request;
        var response = context.Response;
        
        try
        {
            _logger.LogInformation("Received {Method} request: {Url}", request.HttpMethod, request.Url);
            
            if (request.HttpMethod == "GET" && request.Url?.AbsolutePath == "/health")
            {
                await HandleHealthCheckAsync(response);
            }
            else if (request.HttpMethod == "POST" && request.Url?.AbsolutePath.StartsWith("/rooms/") == true && request.Url.AbsolutePath.EndsWith("/shutdown"))
            {
                await HandleShutdownCallbackAsync(request, response);
            }
            else
            {
                response.StatusCode = (int)HttpStatusCode.NotFound;
                response.Close();
            }
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error handling request");
            response.StatusCode = (int)HttpStatusCode.InternalServerError;
            response.Close();
        }
    }
    
    private async Task HandleHealthCheckAsync(HttpListenerResponse response)
    {
        try
        {
            var healthResponse = new { status = "healthy", timestamp = DateTimeOffset.Now.ToUnixTimeSeconds() };
            var jsonResponse = JsonSerializer.Serialize(healthResponse);
            var buffer = Encoding.UTF8.GetBytes(jsonResponse);
            
            response.ContentType = "application/json";
            response.ContentLength64 = buffer.Length;
            response.StatusCode = (int)HttpStatusCode.OK;
            
            await response.OutputStream.WriteAsync(buffer, 0, buffer.Length);
            response.Close();
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error handling health check");
            response.StatusCode = (int)HttpStatusCode.InternalServerError;
            response.Close();
        }
    }
    
    private async Task HandleShutdownCallbackAsync(HttpListenerRequest request, HttpListenerResponse response)
    {
        try
        {
            // Đọc request body
            using var reader = new StreamReader(request.InputStream, request.ContentEncoding ?? Encoding.UTF8);
            var body = await reader.ReadToEndAsync();
            
            _logger.LogInformation("Received shutdown callback: {Body}", body);
            
            var shutdownRequest = JsonSerializer.Deserialize<ShutdownRequest>(body);
            if (shutdownRequest != null)
            {
                _logger.LogInformation("✅ Shutdown callback received: reason={Reason}, at={At}", 
                    shutdownRequest.Reason, shutdownRequest.At);
                
                // Trigger event
                OnShutdownReceived?.Invoke(shutdownRequest.Reason, shutdownRequest.At);
                
                // Send response
                var responseBody = new ShutdownResponse
                {
                    Ok = true,
                    Status = "shutdown_acknowledged"
                };
                
                var jsonResponse = JsonSerializer.Serialize(responseBody);
                var buffer = Encoding.UTF8.GetBytes(jsonResponse);
                
                response.ContentType = "application/json";
                response.ContentLength64 = buffer.Length;
                response.StatusCode = (int)HttpStatusCode.OK;
                
                await response.OutputStream.WriteAsync(buffer, 0, buffer.Length);
                response.Close();
            }
            else
            {
                response.StatusCode = (int)HttpStatusCode.BadRequest;
                response.Close();
            }
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error processing shutdown callback");
            response.StatusCode = (int)HttpStatusCode.InternalServerError;
            response.Close();
        }
    }
    
    public void Stop()
    {
        _cancellationTokenSource.Cancel();
        _httpListener.Stop();
        _logger.LogInformation("Shutdown callback handler stopping...");
    }
    
    public void Dispose()
    {
        Stop();
        _cancellationTokenSource.Dispose();
        _httpListener.Close();
    }
}
