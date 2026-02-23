import SwiftUI

struct WatchMiniCarouselView: View {
    let hourly: [HourlyForecast]
    var body: some View {
        HStack(spacing: 4) {
            ForEach(hourly.prefix(3)) { slot in
                VStack(spacing: 2) {
                    Text(slot.hour, format: .dateTime.hour())
                        .font(.system(size: 8))
                        .foregroundStyle(.white.opacity(0.7))
                    Image(systemName: slot.condition.sfSymbolName)
                        .font(.caption2)
                        .symbolRenderingMode(.hierarchical)
                        .foregroundStyle(.white)
                        .accessibilityLabel(slot.condition.label)
                    Text("\(Int(slot.temperatureFahrenheit.rounded()))°")
                        .font(.caption2)
                        .bold()
                        .foregroundStyle(.white)
                        .monospacedDigit()
                }
                .frame(maxWidth: .infinity)
            }
        }
    }
}

#Preview {
    WatchMiniCarouselView(hourly: HourlyForecast.sampleData)
        .background(Color(hex: "0D1B2A"))
}
