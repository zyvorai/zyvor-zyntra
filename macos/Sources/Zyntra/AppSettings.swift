// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

import Foundation
import SwiftUI

@MainActor
final class AppSettings: ObservableObject {
    private static let packPathKey = "packPath"
    private static let preferBundledSampleKey = "preferBundledSample"
    private static let fixedPortKey = "fixedPort"

    @Published var packPath: String {
        didSet { UserDefaults.standard.set(packPath, forKey: Self.packPathKey) }
    }

    /// When true and packPath is empty, use the sample pack bundled in the app.
    @Published var preferBundledSample: Bool {
        didSet { UserDefaults.standard.set(preferBundledSample, forKey: Self.preferBundledSampleKey) }
    }

    /// Optional fixed port; 0 means pick a free port each launch.
    @Published var fixedPort: Int {
        didSet { UserDefaults.standard.set(fixedPort, forKey: Self.fixedPortKey) }
    }

    @Published var apiKey: String = ""

    init() {
        let defaults = UserDefaults.standard
        self.packPath = defaults.string(forKey: Self.packPathKey) ?? ""
        self.preferBundledSample = defaults.object(forKey: Self.preferBundledSampleKey) as? Bool ?? true
        self.fixedPort = defaults.integer(forKey: Self.fixedPortKey)
        self.apiKey = KeychainStore.loadAPIKey() ?? "dev"
    }

    func saveAPIKey() {
        do {
            try KeychainStore.saveAPIKey(apiKey)
        } catch {
            // Surface via console; settings UI can show status separately.
            NSLog("Zyntra: failed to save API key to Keychain: \(error.localizedDescription)")
        }
    }

    func resolvedPackURL() -> URL? {
        let trimmed = packPath.trimmingCharacters(in: .whitespacesAndNewlines)
        if !trimmed.isEmpty {
            let url = URL(fileURLWithPath: trimmed, isDirectory: true)
            if FileManager.default.fileExists(atPath: url.path) {
                return url
            }
        }
        if preferBundledSample {
            return Bundle.main.resourceURL?.appendingPathComponent("sample-pack", isDirectory: true)
        }
        return nil
    }
}
