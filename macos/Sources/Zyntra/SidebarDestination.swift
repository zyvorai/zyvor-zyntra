// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

import SwiftUI

/// Console hash routes (`/#/<page>`) mirrored in the native sidebar.
enum SidebarDestination: String, CaseIterable, Identifiable, Hashable {
    case overview
    case gaps
    case plan
    case simulate
    case approvals
    case audit
    case signals
    case insights
    case ask
    case model
    case objects
    case scenarios
    case executive

    var id: String { rawValue }

    var title: String {
        switch self {
        case .overview: return "Overview"
        case .gaps: return "Gaps"
        case .plan: return "Plan"
        case .simulate: return "Simulate"
        case .approvals: return "Approvals"
        case .audit: return "Audit"
        case .signals: return "Signals"
        case .insights: return "Insights"
        case .ask: return "Ask Zyntra"
        case .model: return "Model"
        case .objects: return "Objects"
        case .scenarios: return "Scenarios"
        case .executive: return "Executive"
        }
    }

    var systemImage: String {
        switch self {
        case .overview: return "square.grid.2x2"
        case .gaps: return "exclamationmark.triangle"
        case .plan: return "list.number"
        case .simulate: return "arrow.triangle.branch"
        case .approvals: return "checkmark.seal"
        case .audit: return "lock.doc"
        case .signals: return "waveform.path.ecg"
        case .insights: return "chart.line.uptrend.xyaxis"
        case .ask: return "text.bubble"
        case .model: return "point.3.connected.trianglepath.dotted"
        case .objects: return "cube.box"
        case .scenarios: return "rectangle.split.3x1"
        case .executive: return "briefcase"
        }
    }

    var hashPath: String { "/#/\(rawValue)" }

    static let primary: [SidebarDestination] = [
        .overview, .gaps, .plan, .simulate, .approvals, .audit,
    ]

    static let more: [SidebarDestination] = [
        .executive, .signals, .insights, .ask, .model, .objects, .scenarios,
    ]
}
