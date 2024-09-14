package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

type Config struct {
	ffmpegLocation  string
	ffprobeLocation string
	copyFolder      string

	inputFolders []string
}

var config Config

func init() {
	config = linux
	if runtime.GOOS == "windows" {
		config = windows
	}
}

var windows = Config{
	ffmpegLocation:  "C:\\Users\\e19700019\\Downloads\\ffmpeg\\bin\\ffmpeg.exe",
	ffprobeLocation: "C:\\Users\\e19700019\\Downloads\\ffmpeg\\bin\\ffprobe.exe",
	copyFolder:      "Z:",
	inputFolders:    []string{"O:\\NLP\\", "O:\\Learning\\"},
}

var linux = Config{
	ffmpegLocation:  "ffmpeg",
	ffprobeLocation: "ffprobe",
	copyFolder:      "/home/konstantin/temp",
	inputFolders: []string{
		"/cloud/NLP",
		"/cloud/Learning/",
		"/media/konstantin/Hdd2/private",
		"/media/konstantin/Hdd3/video",
		"/media/konstantin/Hdd3/music",
		"/media/konstantin/Hdd1/learning",
	},
}

const (
	ignoreFilename = "ignore"
	maxWidth       = 720 // Width 1024 x Height 724
	maxBitrate     = 128000
)

var ignoreList []string

func ignoreFile(path string) error {
	if len(ignoreList) == 0 {
		dat, err := os.ReadFile(ignoreFilename)
		if err != nil {
			return err
		}
		ignoreList = strings.Split(string(dat), "\n")
	}
	found := false
	for _, suf := range ignoreList {
		if path == suf {
			found = true
		}
	}
	if found {
		return fmt.Errorf("ignored list file")
	}
	return nil
}

func otherFiles(path string) error {
	var list = [...]string{
		".doc", ".docx", ".jpg",
		".txt", ".pdf",
		".png", ".jpeg", ".odg",
		".djvu", ".rtf", ".html",
		".htm", ".flv", ".pptx",
		".zip", ".gif", ".css",
		".exe", ".rar", ".dll",
		".ico", ".url", ".acm",
		".inf", ".ax", ".sub",
		".ccd", ".cue", ".img",
		".ppt", ".epub", ".ax",
		".xml", ".odt", ".wma",
		".fb2", ".ods", ".bbs",
		".vob", ".sfv", ".m3u",
	}
	path = strings.ToLower(path)
	found := false
	for _, suf := range list {
		if strings.HasSuffix(path, suf) {
			found = true
			break
		}
	}
	if found {
		return fmt.Errorf("other file")
	}
	return nil
}

func isVideo(path string) error {
	var list = [...]string{
		".mp4", ".avi", ".mkv",
		".mpg", ".divx",
		".mts", ".wmv",
	}
	path = strings.ToLower(path)
	found := false
	for _, suf := range list {
		if strings.HasSuffix(path, suf) {
			found = true
			break
		}
	}
	if found {
		return nil
	}
	return fmt.Errorf("not video file")
}

func isAudio(path string) error {
	var list = [...]string{
		".mp3", ".flac", ".wav",
	}
	path = strings.ToLower(path)
	found := false
	for _, suf := range list {
		if strings.HasSuffix(path, suf) {
			found = true
			break
		}
	}
	if found {
		return nil
	}
	return fmt.Errorf("not audio file")
}

func ffprobe(path, pr1, pr2 string) (value int, err error) {
	// ffprobe -v error -show_entries stream=width    -of default=noprint_wrappers=1 input.mp4
	// ffprobe -v error -show_entries format=bit_rate -of default=noprint_wrappers=1  input.mp3
	cmd := exec.Command(config.ffprobeLocation,
		"-v", "error",
		"-show_entries", fmt.Sprintf("%s=%s", pr1, pr2),
		"-of", "default=noprint_wrappers=1",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return
	}
	// parse result
	// width=1280

	line := strings.TrimSpace(string(out))
	if !strings.Contains(line, pr2) {
		err = fmt.Errorf("have not %s: `%s`", pr2, string(out))
		return
	}
	if index := strings.Index(line, "="); 0 <= index {
		line = line[index+1:]
	}
	if index := strings.Index(line, "\r"); 0 <= index {
		line = line[:index]
	}
	if index := strings.Index(line, "\n"); 0 <= index {
		line = line[:index]
	}
	value, err = strconv.Atoi(strings.TrimSpace(line))
	return
}

func isVideoValid(path string) error {
	width, err := ffprobe(path, "stream", "width")
	if err != nil {
		return err
	}
	if width < maxWidth {
		return fmt.Errorf("ignore (%05d)", width)
	}
	return nil
}

func isAudioValid(path string) error {
	bitrate, err := ffprobe(path, "format", "bit_rate")
	if err != nil {
		return err
	}
	if bitrate < maxBitrate {
		return fmt.Errorf("ignore (%05d)", bitrate)
	}
	return nil
}

func filesize(path string) (int64, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return -1, err
	}
	// get the size
	return fi.Size(), nil
}

func copy(src, dst string) (err error) {
	// Read all content of src to data, may cause OOM for a large file.
	data, err := ioutil.ReadFile(src)
	if err != nil {
		return
	}
	// Write data to dst
	err = ioutil.WriteFile(dst, data, 0644)
	return
}

func ffmpeg(path string, args ...string) (err error) {
	// filenames
	filename := struct {
		cloudOriginal string
		cloudCompress string

		localCompress string
	}{}
	// get filename
	{
		index := strings.LastIndex(path, string(filepath.Separator))
		if index < 0 {
			panic(path)
		}
		filename.cloudOriginal = path
		// add folders
		filename.cloudCompress = path[:index]
		filename.localCompress = config.copyFolder
		// add name
		name := path[index:] // example: "/file.ext", "\file.ext"
		index = strings.LastIndex(name, ".")
		if index < 0 {
			panic(path)
		}
		// compress size suffix
		name = name[:index] + "_c" + name[index:]
		filename.cloudCompress += name
		filename.localCompress += name
	}
	fmt.Println(filename.cloudOriginal)
	fmt.Println(filename.cloudCompress)
	fmt.Println(filename.localCompress)

	// remove compress file if exist
	fmt.Println("rm loc:", filename.cloudCompress)
	for _, file := range []string{
		filename.cloudCompress,
		filename.localCompress,
	} {
		errLocal := os.Remove(file)
		if errLocal != nil {
			// fmt.Println("A: acceptable error: ", errLocal)
		}
		_ = errLocal // ignore
	}

	defer func() {
		fmt.Println("remove")
		errRem := os.Remove(filename.localCompress)
		if errRem != nil {
			fmt.Fprintf(os.Stdout, "%v", errRem)
		}
	}()

	// compress
	// ffmpeg.exe -i input.mp4 -vf scale=640:-1 out.mp4
	fmt.Println("ffmpeg:", filename.cloudCompress)
	{
		fmt.Printf("[M]")
		fmt.Printf("[M]")
		args = append([]string{
			"-i", filename.cloudOriginal,
		}, append(
			args,
			filename.localCompress)...) // output file
		cmd := exec.Command(config.ffmpegLocation, args...)
		var out []byte
		out, err = cmd.Output()
		if err != nil {
			err = fmt.Errorf("D: %v", err)
			return
		}
		fmt.Println(string(out))
	}

	// copy to cloud
	fmt.Println("copy :", filename.cloudCompress)
	if err = copy(
		filename.localCompress,
		filename.cloudCompress,
	); err != nil {
		err = fmt.Errorf("E: %v", err)
		return
	}

	// remove original
	fmt.Println("remove:", filename.cloudCompress)
	for _, file := range []string{
		filename.cloudOriginal,
	} {
		err = os.Remove(file)
		if err != nil {
			err = fmt.Errorf("G: %v", err)
			return
		}
	}

	// rename
	fmt.Println("rename:", filename.cloudOriginal)
	{
		err = os.Rename(filename.cloudCompress, filename.cloudOriginal)
		if err != nil {
			err = fmt.Errorf("R: %v", err)
			return
		}
	}

	return
}

func convertVideo(path string) (err error) {
	return ffmpeg(path,
		"-vf", fmt.Sprintf("scale=%d:-2", maxWidth), // -2 for divisible by 2
	)
}

func convertAudio(path string) error {
	return ffmpeg(path,
		"-ab", fmt.Sprintf("%d", maxBitrate),
		"-map_metadata", "0",
		"-id3v2_version", "3",
	)
}

func main() {
	// get video files
	input := make(chan string, 10)
	go func() {
		for _, folder := range config.inputFolders {
			if _, err := os.Stat(folder); err != nil {
				if os.IsNotExist(err) {
					// file does not exist
					continue
				}
			}
			err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
				if info.IsDir() {
					return nil
				}
				input <- path
				return nil
			})
			if err != nil {
				// ignore error
				fmt.Fprintf(os.Stderr, "Error in inputFolders: %v", err)
			}
		}
		close(input)
	}()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range input {
				valid := true
				for _, f := range []func(string) error{
					ignoreFile,
					otherFiles,
				} {
					err := f(path)
					if err != nil {
						valid = false
						fmt.Fprintf(os.Stdout, "[1]")
						break
					}
				}
				if !valid {
					continue
				}
				// parse
				var fs []func(string) error
				switch {
				case isVideo(path) == nil:
					fs = []func(string) error{
						isVideoValid,
						convertVideo,
					}
				case isAudio(path) == nil:
					fs = []func(string) error{
						isAudioValid,
						convertAudio,
					}
				}
				ok := true

				before, _ := filesize(path)
				for _, f := range fs { // TODO save filename
					err := f(path)
					if err != nil {
						fmt.Fprintf(os.Stdout, "%s:%v\n", path, err)
						ok = false
						break
					}
				}
				after, _ := filesize(path)
				if 0 < len(fs) && ok {
					fmt.Fprintf(os.Stdout, "%s:SUCCESS:%d:%d:%.5f\n", path, before, after, float64(before)/float64(after))
					fmt.Println("-----------------")
					ignoreList = append(ignoreList, path)

					err := os.WriteFile(
						ignoreFilename,
						[]byte(strings.Join(ignoreList, "\n")),
						0644,
					)
					if err != nil {
						fmt.Println("Ignore list cannot be write:", err)
					}
				}
			}
		}()
	}
	wg.Wait()
}
