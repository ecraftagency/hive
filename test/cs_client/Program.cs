using System;
using System.Threading.Tasks;
using CsClient;

class Program
{
	static async Task Main(string[] args)
	{
		Console.OutputEncoding = System.Text.Encoding.UTF8;
		var mode = args.Length > 0 ? args[0].Trim().ToLowerInvariant() : "integrate";
		Console.WriteLine($"🚀 cs_client starting, mode={mode}\n");
		switch (mode)
		{
			case "stress":
				await StressFlow.Run();
				break;
			case "integrate":
			default:
				await IntegrateFlow.Run();
				break;
		}
	}
}
