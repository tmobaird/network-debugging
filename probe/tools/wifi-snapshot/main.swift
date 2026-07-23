import CoreWLAN
import Foundation

// WiFiSnapshot is the JSON contract consumed by the Go probe. Optional fields
// may be absent when Wi-Fi is disconnected, Core WLAN cannot read a value, or
// macOS privacy controls hide SSID/BSSID from this process.
struct WiFiSnapshot: Encodable {
    let interface: String
    let ssid: String?
    let bssid: String?
    let channelNumber: Int?
    let band: String?
    let channelWidthMHz: Int?
    let signalDBM: Int?
    let noiseDBM: Int?
}

func bandName(_ band: CWChannelBand) -> String? {
    switch band {
    case .band2GHz:
        return "2.4GHz"
    case .band5GHz:
        return "5GHz"
    case .band6GHz:
        return "6GHz"
    default:
        return nil
    }
}

func channelWidthMHz(_ width: CWChannelWidth) -> Int? {
    switch width {
    case .width20MHz:
        return 20
    case .width40MHz:
        return 40
    case .width80MHz:
        return 80
    case .width160MHz:
        return 160
    default:
        return nil
    }
}

func fail(_ message: String) -> Never {
    FileHandle.standardError.write(Data("wifi-snapshot: \(message)\n".utf8))
    exit(EXIT_FAILURE)
}

let requestedInterface = CommandLine.arguments.dropFirst().first
let client = CWWiFiClient.shared()

let wifiInterface: CWInterface?
if let requestedInterface {
    wifiInterface = client.interface(withName: requestedInterface)
} else {
    wifiInterface = client.interface()
}

guard let wifiInterface else {
    fail("no Wi-Fi interface found")
}

let channel = wifiInterface.wlanChannel()
let rawSignal = wifiInterface.rssiValue()
let rawNoise = wifiInterface.noiseMeasurement()

// Core WLAN returns zero when RSSI/noise is unavailable. Real Wi-Fi readings
// are negative dBm values, so expose unavailable readings as absent JSON fields.
let snapshot = WiFiSnapshot(
    interface: wifiInterface.interfaceName ?? requestedInterface ?? "",
    ssid: wifiInterface.ssid(),
    bssid: wifiInterface.bssid(),
    channelNumber: channel?.channelNumber,
    band: channel.flatMap { bandName($0.channelBand) },
    channelWidthMHz: channel.flatMap { channelWidthMHz($0.channelWidth) },
    signalDBM: rawSignal == 0 ? nil : rawSignal,
    noiseDBM: rawNoise == 0 ? nil : rawNoise
)

let encoder = JSONEncoder()
encoder.outputFormatting = [.prettyPrinted, .sortedKeys]

do {
    let data = try encoder.encode(snapshot)
    FileHandle.standardOutput.write(data)
    FileHandle.standardOutput.write(Data("\n".utf8))
} catch {
    fail("encode JSON: \(error)")
}
