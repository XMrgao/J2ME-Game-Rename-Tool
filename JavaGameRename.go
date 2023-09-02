package main

import (
	"archive/zip"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/spf13/viper"
)

type JavaGame struct {
	MIDletName        string
	MIDletDescription string
}

func VisitDir(path string, fn func(string)) {
	fileInfoArray, _ := os.ReadDir(path)
	base := path + string(os.PathSeparator)
	for _, info := range fileInfoArray {
		if info.IsDir() {
			VisitDir(base+info.Name(), fn)
			continue
		}
		fn(base + info.Name())
	}
}

func ReadJarMetaInfo(jarPath string) *JavaGame {
	defer func() {
		if err := recover(); err != nil {
			log.Println(jarPath + "出错了！")
		}
	}()
	reader, err := zip.OpenReader(jarPath)
	if err != nil {
		log.Println(jarPath + "\t" + err.Error())
		return nil
	}
	defer reader.Close()
	for _, f := range reader.File {
		if f.Name == "META-INF/MANIFEST.MF" {
			rc, err := f.Open()
			if err != nil {
				log.Println(jarPath + "\t" + err.Error())
			}
			defer rc.Close()
			buf := make([]byte, f.UncompressedSize)
			_, err = io.ReadFull(rc, buf)
			if err != nil {
				log.Println(jarPath + "\t" + err.Error())
			}
			content := string(buf)
			contentMap := map[string]string{}

			for _, line := range strings.Split(content, "\n") {
				if strings.Trim(line, " \r\n") == "" {
					continue
				}
				index := strings.Index(line, ":")
				if index == -1 {
					continue
				}
				key := line[0:index]
				value := line[index+1:]
				contentMap[strings.Trim(key, " \r\n")] = strings.Trim(value, " \r\n")
			}

			name := NormalizationName(contentMap["MIDlet-Name"])
			if !HasChineseChar(name) {
				tmp := NormalizationName(contentMap["MIDlet-1"])
				arr := strings.Split(tmp, ",")
				if len(arr) > 0 {
					tmp = NormalizationName(arr[0])
					if HasChineseChar(tmp) {
						name = tmp
					}
				}
			}
			desc := NormalizationName(contentMap["MIDlet-Description"])
			return &JavaGame{MIDletName: name, MIDletDescription: desc}
		}
	}
	return nil
}

func HasChineseChar(str string) bool {
	for _, r := range str {
		if unicode.Is(unicode.Scripts["Han"], r) || (regexp.MustCompile("[\u3002\uff1b\uff0c\uff1a\u201c\u201d\uff08\uff09\u3001\uff1f\u300a\u300b]").MatchString(string(r))) {
			return true
		}
	}
	return false
}

func NormalizationName(str string) string {
	str = strings.ReplaceAll(str, "MIDlet-Name:", "")
	str = strings.ReplaceAll(str, "MIDlet-Description:", "")
	str = strings.Trim(str, " \r\n\\:")
	str = strings.ReplaceAll(str, ":", "")
	str = strings.ReplaceAll(str, "?", "")
	str = strings.ReplaceAll(str, "*", "")
	return str
}

func CheckErr(err error) {
	if err != nil {
		panic(err)
	}
}

const (
	ConfigFile            = "config.txt"
	DefaultExportFileName = "Java游戏目录.csv"
	DefaultRename         = "0"
	DefaultJavaGameDir    = "."
)

func main() {
	viper.SetConfigName(ConfigFile)
	viper.AddConfigPath(".")
	viper.SetConfigType("properties")

	viper.SetDefault("ExportFile", DefaultExportFileName)
	viper.SetDefault("Rename", DefaultRename)
	viper.SetDefault("JavaGameDir", DefaultJavaGameDir)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			os.WriteFile(ConfigFile, []byte("JavaGameDir="+DefaultJavaGameDir+"\nExportFile="+DefaultExportFileName+"\nRename="+DefaultRename), 0777)
		} else {
			panic(err)
		}
	}

	file, err := os.Create(viper.GetString("ExportFile"))
	CheckErr(err)
	defer file.Close()
	file.WriteString("\xEF\xBB\xBF")

	writer := csv.NewWriter(file)
	writer.Write([]string{"文件路径", "游戏名字", "游戏描述"})
	rename := viper.GetInt("Rename") == 1

	VisitDir(viper.GetString("JavaGameDir"), func(filePath string) {
		index := strings.LastIndex(filePath, ".")
		if index == -1 {
			return
		}
		suffix := filePath[index:]
		suffix = strings.ToLower(suffix)
		if suffix != ".jar" {
			return
		}
		game := ReadJarMetaInfo(filePath)
		if game == nil {
			writer.Write([]string{filePath, "!!!!!文件错误，读取信息失败!!!!!!", ""})
			return
		}
		//fmt.Println(filePath, game.MIDletName, game.MIDletDescription)
		writer.Write([]string{filePath, game.MIDletName, game.MIDletDescription})
		if rename {
			err := os.Rename(filePath, filepath.Dir(filePath)+string(os.PathSeparator)+game.MIDletName+".jar")
			if err != nil {
				fmt.Println(err)
			}
		}
	})
	writer.Flush()
	fmt.Println("游戏目录生成结束！文件路径:" + viper.GetString("ExportFile"))
}
