import SwiftUI

@main
struct SkiesTVApp: App {
    @StateObject private var store = WeatherStore.shared
    var body: some Scene {
        WindowGroup {
            TVRootView()
                .environmentObject(store)
        }
    }
}
