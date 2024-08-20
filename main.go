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
	inputFolders:    []string{"/cloud/NLP", "/cloud/Learning/"},
}

const (
	ignoreFile = "ignore"
	maxWidth   = 720 // Width 1024 x Height 724
)

var (
	ignoreExt = [...]string{
		".doc", ".docx", ".jpg",
		".txt", ".mp3", ".pdf",
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
		".fb2", ".ods", "bbs",
		".vob",
	}
	videoExt = [...]string{
		".mp4", ".avi", ".mkv",
		".mpg", ".divx",
		".mts", ".wmv",
	}
)

func main() {
	config := linux
	if runtime.GOOS == "windows" {
		config = windows
	}

	var ignoreList []string
	dat, err := os.ReadFile(ignoreFile)
	if err != nil {
		fmt.Println(">>>>>>>>> cannot read ignore list", err)
	} else {
		ignoreList = strings.Split(string(dat), "\n")
	}

	// get video files
	input := make(chan string, 50)
	go func() {
		for _, folder := range config.inputFolders {
			err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
				if info.IsDir() {
					return nil
				}
				{
					found := false
					for _, suf := range ignoreList {
						if path == suf {
							found = true
						}
					}
					if found {
						fmt.Printf("1")
						return nil
					}
				}
				{
					found := false
					for _, suf := range ignoreExt {
						if strings.HasSuffix(strings.ToLower(path), suf) {
							found = true
						}
					}
					if found {
						fmt.Printf("2")
						return nil
					}
				}
				{
					found := false
					for _, suf := range videoExt {
						if strings.HasSuffix(strings.ToLower(path), suf) {
							found = true
						}
					}
					if found {
						input <- path
						return nil
					}
				}
				return nil
			})
			if err != nil {
				panic(err)
			}
		}
		close(input)
	}()

	var errs []error

	// filter of video size
	filter := make(chan string, 20)
	ignoreVideo := make(chan string, 20)
	const size = 10
	var wg sync.WaitGroup
	wg.Add(size)
	for i := 0; i < size; i++ {
		go func() {
			defer wg.Done()
			for file := range input {
				// ffprobe -v error -show_entries stream=width,height -of default=noprint_wrappers=1 input.mp4
				fmt.Printf("[P]")
				cmd := exec.Command(config.ffprobeLocation,
					"-v", "error",
					"-show_entries", "stream=width", // ONLY WIDTH
					"-of", "default=noprint_wrappers=1",
					file,
				)
				out, err := cmd.Output()
				if err != nil {
					errs = append(errs, err)
					continue
				}
				// parse result
				// width=1280
				// height=720

				line := strings.TrimSpace(string(out))

				if !strings.Contains(line, "width") {
					errs = append(errs, fmt.Errorf("have not width: `%s`", string(out)))
					continue
				}
				if index := strings.Index(line, "="); 0 <= index {
					line = line[index+1:]
				}
				if index := strings.Index(line, "\r"); 0 <= index {
					line = line[:index]
				}
				width, err := strconv.Atoi(strings.TrimSpace(line))
				if err != nil {
					errs = append(errs, fmt.Errorf("%v -- %s", err, line))
					continue
				}
				if maxWidth < width {
					fmt.Printf("add    (%05d): %s\n", width, file)
					filter <- file
				} else {
					fmt.Printf("ignore (%05d): %s\n", width, file)
					ignoreVideo <- file
				}
			}
		}()
	}
	go func() {
		counter := 0
		for file := range ignoreVideo {
			ignoreList = append(ignoreList, file)
			counter++
			if 10 < counter {
				counter = 0
				// save ignore list
				err := os.WriteFile(
					ignoreFile,
					[]byte(strings.Join(ignoreList, "\n")),
					0644,
				)
				if err != nil {
					fmt.Println("Ignore list cannot be write:", err)
				}
			}
		}
	}()
	go func() {
		wg.Wait()
		close(filter)
		close(ignoreVideo)
	}()

	// compress
	var fok []string // file is ok
	for f := range filter {
		if err := action(config, f); err != nil {
			errs = append(errs, fmt.Errorf("%v --- %v", f, err))
			continue
		}
		fok = append(fok, f)
	}
	for i := range fok {
		fmt.Println("ok", i, fok[i])
	}
	fmt.Println("---------------------")
	for i := range errs {
		fmt.Println(i, errs[i])
	}
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

func action(config Config, f string) (err error) {
	defer func() {
		if err != nil {
			fmt.Println("++++++++++++++++++++")
			fmt.Printf("Error:\n%s\n%v\n", f, err)
			fmt.Println("++++++++++++++++++++")
		}
	}()
	filename := struct { // filenames
		cloudOriginal string
		cloudCompress string

		// localOriginal string
		localCompress string
	}{}
	// get filename
	{
		index := strings.LastIndex(f, string(filepath.Separator))
		if index < 0 {
			panic(f)
		}
		filename.cloudOriginal = f[:index]
		filename.cloudCompress = f[:index]
		filename.localCompress = config.copyFolder

		name := f[index:]
		filename.cloudOriginal += name

		part := name
		index = strings.LastIndex(part, ".")
		if index < 0 {
			panic(f)
		}
		name = part[:index] +
			// compress size suffix
			fmt.Sprintf("_c%d", maxWidth) +
			part[index:]
		filename.cloudCompress += name
		filename.localCompress += name
	}
	fmt.Println(filename.cloudOriginal)
	fmt.Println(filename.cloudCompress)
	fmt.Println(filename.localCompress)
	fmt.Println("-----------------")

	// remove compress file if exist
	fmt.Println("rm loc:", filename.cloudCompress)
	for _, file := range []string{
		filename.cloudCompress,
		filename.localCompress,
	} {
		errLocal := os.Remove(file)
		if errLocal != nil {
			fmt.Println("A: acceptable error: ", errLocal)
		}
	}

	// copy to specific folder
	// 	fmt.Println("copy  :", filename.cloudCompress)
	// 	{
	// 		cmd := exec.Command("cp",
	// 			filename.cloudOriginal,
	// 		)
	// 		var out []byte
	// 		out, err = cmd.Output()
	// 		if err != nil {
	// 			err = fmt.Errorf("B: %v", err)
	// 			return
	// 		}
	// 		fmt.Println(string(out))
	// 	}

	defer func() {
		fmt.Println("remove:", filename.cloudCompress)
		for _, file := range []string{
			filename.localCompress,
		} {
			err2 := os.Remove(file)
			if err2 != nil {
				err = fmt.Errorf("%v :: %v", err, err2)
				err = fmt.Errorf("C: %v", err)
			}
		}
	}()

	// compress
	// ffmpeg.exe -i input.mp4 -vf scale=640:-1 out.mp4
	fmt.Println("ffmpeg:", filename.cloudCompress)
	{
		fmt.Printf("[M]")
		args := []string{
			"-i", filename.cloudOriginal, // fmt.Sprintf("'%s'", filename.cloudOriginal),
			"-vf", fmt.Sprintf("scale=%d:-2", maxWidth), // -2 for divisible by 2
			filename.localCompress, // fmt.Sprintf("'%s'", filename.localCompress),
		}
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

	fmt.Println("--- SUCCESS ---", filename.cloudCompress)
	return
}
