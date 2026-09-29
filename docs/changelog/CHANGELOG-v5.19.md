# JTTY-Go v5.19 UI / logging revision

- Decode list now shows UTC, Freq and message only; DT/SNR removed.
- Removed the bypass control and the large green RX indicator around the main dial display.
- Audio settings follow the WSJT-X-style device/settings layout.
- Added optional WAV recording, configurable on the audio page; blank path defaults to `%LOCALAPPDATA%\\JTTY-Go\\record`; recordings are split at UTC midnight.
- Added a dedicated 保存 menu for enabling/disabling ALL.txt decode logging and record logging, with the old logging settings page removed.
- Removed automatic ADIF logging from TX-finished automation.
- Added manual QSO logging: the QSO start timestamp is the first information send for the selected DX call; the end timestamp is captured when the operator clicks 记录通联; UTC is used for both.
