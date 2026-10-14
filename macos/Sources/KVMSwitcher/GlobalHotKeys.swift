import Carbon.HIToolbox
import Foundation

/// Registers system-wide ⌃⌥⌘1 … ⌃⌥⌘N hotkeys via Carbon, which needs no Accessibility permission.
final class GlobalHotKeys {
    static let modifierDescription = "⌃⌥⌘"
    private static let hotKeySignature: OSType = 0x4B56_4D53 // "KVMS"
    private static let numberKeyCodes = [
        kVK_ANSI_1, kVK_ANSI_2, kVK_ANSI_3, kVK_ANSI_4,
        kVK_ANSI_5, kVK_ANSI_6, kVK_ANSI_7, kVK_ANSI_8,
    ]

    private let onPortHotKey: (Int) -> Void
    private var registeredHotKeys: [EventHotKeyRef] = []
    private var eventHandlerRef: EventHandlerRef?

    init(onPortHotKey: @escaping (Int) -> Void) {
        self.onPortHotKey = onPortHotKey
    }

    deinit {
        unregisterAll()
    }

    func register(portCount: Int) {
        unregisterAll()
        installEventHandlerIfNeeded()
        let modifierFlags = UInt32(controlKey | optionKey | cmdKey)
        for portIndex in 0 ..< min(portCount, Self.numberKeyCodes.count) {
            let hotKeyId = EventHotKeyID(signature: Self.hotKeySignature, id: UInt32(portIndex))
            var hotKeyRef: EventHotKeyRef?
            let status = RegisterEventHotKey(
                UInt32(Self.numberKeyCodes[portIndex]), modifierFlags, hotKeyId,
                GetApplicationEventTarget(), 0, &hotKeyRef
            )
            if status == noErr, let hotKeyRef { registeredHotKeys.append(hotKeyRef) }
        }
    }

    func unregisterAll() {
        registeredHotKeys.forEach { UnregisterEventHotKey($0) }
        registeredHotKeys.removeAll()
    }

    private func installEventHandlerIfNeeded() {
        guard eventHandlerRef == nil else { return }
        var pressedEventType = EventTypeSpec(
            eventClass: OSType(kEventClassKeyboard), eventKind: UInt32(kEventHotKeyPressed)
        )
        let selfPointer = Unmanaged.passUnretained(self).toOpaque()
        InstallEventHandler(
            GetApplicationEventTarget(),
            { _, eventRef, userData in
                guard let eventRef, let userData else { return OSStatus(eventNotHandledErr) }
                var hotKeyId = EventHotKeyID()
                let status = GetEventParameter(
                    eventRef, EventParamName(kEventParamDirectObject), EventParamType(typeEventHotKeyID),
                    nil, MemoryLayout<EventHotKeyID>.size, nil, &hotKeyId
                )
                guard status == noErr, hotKeyId.signature == GlobalHotKeys.hotKeySignature else {
                    return OSStatus(eventNotHandledErr)
                }
                let hotKeys = Unmanaged<GlobalHotKeys>.fromOpaque(userData).takeUnretainedValue()
                hotKeys.onPortHotKey(Int(hotKeyId.id))
                return noErr
            },
            1, &pressedEventType, selfPointer, &eventHandlerRef
        )
    }
}
