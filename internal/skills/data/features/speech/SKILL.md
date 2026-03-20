---
name: "speech"
description: "Speech recognition: SFSpeechRecognizer, live and file-based recognition, permissions. Use when implementing app features related to speech."
---
# Speech

SPEECH RECOGNITION:
- import Speech; SFSpeechRecognizer()
- Requires NSSpeechRecognitionUsageDescription + NSMicrophoneUsageDescription (add CONFIG_CHANGES)
- SFSpeechRecognizer.requestAuthorization() for permission
- On-device: SFSpeechRecognizer(locale:), set requiresOnDeviceRecognition = true
- Live: SFSpeechAudioBufferRecognitionRequest + AVAudioEngine
- File: SFSpeechURLRecognitionRequest(url:)
