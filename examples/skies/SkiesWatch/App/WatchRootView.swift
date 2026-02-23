import SwiftUI

struct WatchRootView: View {
    @EnvironmentObject var store: WeatherStore
    var body: some View {
        WatchMainView(snapshot: store.snapshot)
    }
}

#Preview {
    WatchRootView()
        .environmentObject(WeatherStore.shared)
}
