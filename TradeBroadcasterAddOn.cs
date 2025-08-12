#region Using declarations
using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;
using System.Net.Http;
using System.Threading;
using System.Threading.Tasks;
using Newtonsoft.Json;
using NinjaTrader.Cbi;
using NinjaTrader.Gui;
using NinjaTrader.NinjaScript;
using System.Windows; 
#endregion

namespace NinjaTrader.NinjaScript.AddOns
{
    public class TradeBroadcasterAddOn : AddOnBase
    {
        private static readonly HttpClient httpClient = new HttpClient();
        private string endpointUrl = "http://localhost:8080/webhook";

        // NEW: More stable, lightweight throttling mechanism
        private int updateScheduled = 0; // 0 for false, 1 for true
        private const int ThrottleTimeMs = 250; // Send updates at most every 250ms

        protected override void OnStateChange()
        {
            if (State == State.Active)
            {
                Account.All.ToList().ForEach(acc =>
                {
                    if (acc.Connection != null && acc.Connection.Status == ConnectionStatus.Connected)
                    {
                        acc.OrderUpdate += OnAccountEvent;
                        acc.ExecutionUpdate += OnAccountEvent;
                        acc.PositionUpdate += OnAccountEvent;
                        acc.AccountItemUpdate += OnAccountEvent;
                    }
                });
                PrintOutput("TradeBroadcaster AddOn started and subscribed to account events.");
                OnAccountEvent(null, null); // Send an initial snapshot
            }
            else if (State == State.Terminated)
            {
                Account.All.ToList().ForEach(acc =>
                {
                    acc.OrderUpdate -= OnAccountEvent;
                    acc.ExecutionUpdate -= OnAccountEvent;
                    acc.PositionUpdate -= OnAccountEvent;
                    acc.AccountItemUpdate -= OnAccountEvent;
                });
            }
        }

        // NEW: Replaced the Timer with a more robust Task-based throttling mechanism
        private void OnAccountEvent(object sender, EventArgs e)
        {
            // Atomically check if an update is already scheduled. If not, schedule one.
            if (Interlocked.CompareExchange(ref updateScheduled, 1, 0) == 0)
            {
                Task.Run(async () =>
                {
                    // Wait for the throttle period
                    await Task.Delay(ThrottleTimeMs);
                    try
                    {
                        // Send a snapshot for every connected account
                        foreach (var acc in Account.All)
                        {
                            if (acc.Connection != null && acc.Connection.Status == ConnectionStatus.Connected)
                            {
                                // We are already on a background thread from Task.Run, so we can await directly
                                await SendFullSnapshot(acc);
                            }
                        }
                    }
                    catch (Exception ex)
                    {
                        PrintOutput($"Unhandled exception in update task: {ex.Message}");
                    }
                    finally
                    {
                        // After sending, allow a new update to be scheduled
                        Interlocked.Exchange(ref updateScheduled, 0);
                    }
                });
            }
        }

        private async Task SendFullSnapshot(Account account)
        {
            try
            {
                var activeOrderStates = new[] { OrderState.Accepted, OrderState.Working, OrderState.Submitted };
                var snapshot = new
                {
                    timestamp = DateTime.UtcNow,
                    account = account.Name,
                    balance = account.Get(AccountItem.CashValue, Currency.UsDollar),
                    realized = account.Get(AccountItem.RealizedProfitLoss, Currency.UsDollar),
                    unrealized = account.Get(AccountItem.UnrealizedProfitLoss, Currency.UsDollar),
                    positions = account.Positions.Select(p => new
                    {
                        instrument = p.Instrument.FullName,
                        symbol = p.Instrument.MasterInstrument.Name,
                        marketPosition = p.MarketPosition.ToString(),
                        quantity = p.Quantity,
                        averagePrice = p.AveragePrice,
                        unrealized = p.GetUnrealizedProfitLoss(PerformanceUnit.Currency),
                        currentPrice = p.Instrument.MarketData.Last?.Price ?? 0
                    }).ToList(),
                    workingOrders = account.Orders
                        .Where(o => activeOrderStates.Contains(o.OrderState))
                        .Select(o => new
                        {
                            orderId = o.OrderId,
                            instrument = o.Instrument.FullName,
                            orderType = o.OrderType.ToString(),
                            orderAction = o.OrderAction.ToString(),
                            quantity = o.Quantity,
                            filled = o.Filled,
                            limitPrice = o.LimitPrice,
                            stopPrice = o.StopPrice,
                            state = o.OrderState.ToString(),
                            name = o.Name,
                            oco = o.Oco,
                            isStopLoss = o.Name == "Stop loss",
                            isProfitTarget = o.Name == "Profit target"
                        }).ToList()
                };
                string json = JsonConvert.SerializeObject(snapshot);
                await PostJsonAsync(json);
            }
            catch (Exception ex)
            {
                PrintOutput($"Error in SendFullSnapshot: {ex.Message}");
            }
        }

        private async Task PostJsonAsync(string json)
        {
            try
            {
                var content = new StringContent(json, Encoding.UTF8, "application/json");
                var response = await httpClient.PostAsync(endpointUrl, content);
                if (!response.IsSuccessStatusCode)
                {
                    PrintOutput($"HTTP Error: {response.StatusCode}");
                }
            }
            catch (Exception ex)
            {
                PrintOutput("HTTP Post error: " + ex.Message);
            }
        }
        
        private void PrintOutput(string message)
        {
            if (Application.Current != null && Application.Current.Dispatcher != null)
            {
                Application.Current.Dispatcher.InvokeAsync(() => NinjaTrader.Code.Output.Process(message, PrintTo.OutputTab1));
            }
        }
    }
}