package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const chanSize = 10

func main() {
	var (
		ffmpegLocation  = "C:\\Users\\e19700019\\Downloads\\ffmpeg\\bin\\ffmpeg.exe"
		ffprobeLocation = "C:\\Users\\e19700019\\Downloads\\ffmpeg\\bin\\ffprobe.exe"
		folders         = []string{"O:\\NLP\\", "O:\\Learning\\"}
		copyFolder      = "Z:"
		maxWidth        = 1024 // Width 1024 x Height 724
	)
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
	}
	video := []string{
		".mp4", ".avi", ".mkv",
		".mpg", ".vob", ".divx",
		".mts", ".wmv",
	}

	// get video files
	input := make(chan string, chanSize)
	go func() {
		for _, folder := range folders {
			err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
				if info.IsDir() {
					return nil
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

	// filter of video size
	filter := make(chan string, chanSize*10)
	go func() {
		for file := range input {
			// ffprobe -v error -show_entries stream=width,height -of default=noprint_wrappers=1 input.mp4
			cmd := exec.Command(ffprobeLocation,
				"-v", "error",
				"-show_entries", "stream=width,height",
				"-of", "default=noprint_wrappers=1",
				file,
			)
			stdout, err := cmd.Output()
			if err != nil {
				panic(err)
			}
			// parse result
			// width=1280
			// height=720
			lines := strings.Split(string(stdout), "\n")
			var height, width int
			for _, line := range lines {
				if !strings.Contains(line, "width") {
					continue
				}
				index := strings.Index(line, "=")
				if index < 0 {
					continue
				}
				width, err := strconv.Atoi(strings.TrimSpace(line[index+1:]))
				if err != nil {
					panic(err)
				}
				if maxWidth < width {
					filter <- file
					fmt.Printf("add    (%05d): %s\n", width, file)
					break
				} else {
					fmt.Printf("ignore (%05d): %s\n", width, file)
				}
			}
		}
		close(filter)
	}()

	// compress
	for f := range filter {
		filename := struct { // filenames
			cloudOriginal string
			cloudCompress string

			localOriginal string
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
			filename.localOriginal = copyFolder
			filename.localCompress = copyFolder

			name := f[index:]
			filename.cloudOriginal += name
			filename.localOriginal += name

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
		fmt.Println(filename.localOriginal)
		fmt.Println(filename.localCompress)
		fmt.Println("-----------------")

		// remove compress file if exist
		fmt.Println("rm loc:", filename.cloudCompress)
		for _, file := range []string{
			filename.cloudCompress,
			filename.localOriginal,
			filename.localCompress,
		} {
			err := os.Remove(file)
			if err != nil {
				fmt.Println("not exist file: ", file)
			}
		}

		// copy to specific folder
		fmt.Println("copy  :", filename.cloudCompress)
		{
			cmd := exec.Command("cp",
				filename.cloudOriginal,
				filename.localOriginal,
			)
			out, err := cmd.Output()
			if err != nil {
				panic(err)
			}
			fmt.Println(string(out))
		}

		// compress
		// ffmpeg.exe -i input.mp4 -vf scale=640:-1 out.mp4
		fmt.Println("ffmpeg:", filename.cloudCompress)
		{
			cmd := exec.Command(ffmpegLocation,
				"-i", filename.localOriginal,
				"-vf", "scale=640:-1",
				filename.localCompress,
			)
			out, err := cmd.Output()
			if err != nil {
				panic(err)
			}
			fmt.Println(string(out))
		}

		// copy to cloud
		fmt.Println("cp    :", filename.cloudCompress)
		{
			cmd := exec.Command("cp",
				filename.localCompress,
				filename.cloudCompress,
			)
			out, err := cmd.Output()
			if err != nil {
				panic(err)
			}
			fmt.Println(string(out))
		}

		// remove original file
		fmt.Println("remove:", filename.cloudCompress)
		for _, file := range []string{
			filename.localOriginal,
			filename.localCompress,
			filename.cloudOriginal,
		} {
			err := os.Remove(file)
			if err != nil {
				fmt.Println("not exist file: ", file)
			}
		}
	}
}
