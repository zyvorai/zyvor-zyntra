// swift-tools-version: 5.9
// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

import PackageDescription

let package = Package(
    name: "Zyntra",
    platforms: [.macOS(.v14)],
    products: [
        .executable(name: "Zyntra", targets: ["Zyntra"]),
    ],
    targets: [
        .executableTarget(
            name: "Zyntra",
            path: "Sources/Zyntra",
            linkerSettings: [
                .linkedFramework("SwiftUI"),
                .linkedFramework("AppKit"),
                .linkedFramework("WebKit"),
                .linkedFramework("Security"),
            ]
        ),
    ]
)
