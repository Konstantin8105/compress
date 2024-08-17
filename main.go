package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const (
	ffmpegLocation  = "C:\\Users\\e19700019\\Downloads\\ffmpeg\\bin\\ffmpeg.exe"
	ffprobeLocation = "C:\\Users\\e19700019\\Downloads\\ffmpeg\\bin\\ffprobe.exe"
	copyFolder      = "Z:"
	ignoreFile      = "ignore"
	maxWidth        = 720 // 1024 // Width 1024 x Height 724
)

func main() {
	folders := []string{"O:\\NLP\\", "O:\\Learning\\"}
	var ignoreList []string
	ignore := []string{
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
	video := []string{
		".mp4", ".avi", ".mkv",
		".mpg", ".divx",
		".mts", ".wmv",
	}
	dat, err := os.ReadFile(ignoreFile)
	if err != nil {
		fmt.Println(">>>>>>>>> cannot read ignore list", err)
	} else {
		ignoreList = strings.Split(string(dat), "\n")
	}

	// get video files
	input := make(chan string, 20)
	go func() {
		for _, folder := range folders {
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
						return nil
					}
				}
				{
					found := false
					for _, suf := range ignore {
						if strings.HasSuffix(strings.ToLower(path), suf) {
							found = true
						}
					}
					if found {
						return nil
					}
				}
				{
					found := false
					for _, suf := range video {
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
				cmd := exec.Command(ffprobeLocation,
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
		if err := action(f); err != nil {
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

func action(f string) (err error) {
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
		index := strings.LastIndex(f, "\\")
		if index < 0 {
			panic(f)
		}
		filename.cloudOriginal = f[:index]
		filename.cloudCompress = f[:index]
		filename.localCompress = copyFolder

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
		args := []string{
			"-i", filename.cloudOriginal, // fmt.Sprintf("'%s'", filename.cloudOriginal),
			"-vf", fmt.Sprintf("scale=%d:-1", maxWidth),
			filename.localCompress, // fmt.Sprintf("'%s'", filename.localCompress),
		}
		cmd := exec.Command(ffmpegLocation, args...)
		var out []byte
		out, err = cmd.Output()
		if err != nil {
			err = fmt.Errorf("D: %v", err)
			return
		}
		fmt.Println(string(out))
	}

	// copy to cloud
	fmt.Println("copy    :", filename.cloudCompress)
	{
		var input []byte
		input, err = ioutil.ReadFile(filename.localCompress)
		if err != nil {
			return
		}
		err = ioutil.WriteFile(filename.cloudCompress, input, 0644)
		if err != nil {
			return
		}
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
