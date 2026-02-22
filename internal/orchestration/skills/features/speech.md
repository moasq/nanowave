---
name: speech
description: "Speech recognition: SFSpeechRecognizer, live and file-based recognition, permissions."
user-invocable: false
---
# Speech

SPEECH RECOGNITION:
- import Speech; SFSpeechRecognizer()
- Requires NSSpeechRecognitionUsageDescription + NSMicrophoneUsageDescription (add CONFIG_CHANGES)
- SFSpeechRecognizer.requestAuthorization() for permission
- On-device: SFSpeechRecognizer(locale:), set requiresOnDeviceRecognition = true
- Live: SFSpeechAudioBufferRecognitionRequest + AVAudioEngine
- File: SFSpeechURLRecognitionRequest(url:)
