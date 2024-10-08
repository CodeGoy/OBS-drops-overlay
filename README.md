# obs-drops-overlay

A obs-browser-source overlay for playing sounds, music and Videos right in OBS.
Controlled by a webserver the overlay can be controlled from a tablet or phone.

## Functionality

### Systray

```shell
Flags:
  -path string
        override file location
  -port string
        port to listen on (default "8605")
  -systray
        show systray 

```

### Video player
* provides a transparent video player
* when a video ends the overlay becomes transparent
* remote play/pause and seek
* control playbackRate from 0.2 - 2.0 (in 0.2 increments)
* loads local files and urls

### Video Loop
* drop videos can be played while looping video
* plays all videos in videoLoop directory in a loop forever
* loop continues until stop is pressed

### Audio player
* plays mp3 files
* volume control effects all playing sounds

### Music player
* has separate volume control

### Supported Formats

| type  | format  |
|-------|---------|
| audio | mp3     |
| video | mkv mp4 |

## adding to obs

---

Add a browser source

![.git_assets/add-browser-source.png](.git_assets/add-browser-source.png)

---

Add the overlay url in console output to browser source in obs (http://xxx.xxx.xxx.xxx:8605/overlay)

![](.git_assets/set-values.png)
