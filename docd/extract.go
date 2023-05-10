package main

import (
	"fmt"
	"github.com/wangbin/jiebago/posseg"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
)

var (
	jieba posseg.Segmenter
)

func isExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

func init() {
	defaultDictName := "default.dic"
	dictList := []string{defaultDictName, "userdict.txt"}
	for _, dict := range dictList {
		if isExists(dict) {
			jieba.LoadDictionary(dict)
		} else if dict == defaultDictName {
			jieba.LoadDictionary("dict.txt")
		}
	}
}

func first(array []string) string {
	if len(array) > 0 {
		return array[0]
	}
	return ""
}

func toArray(ch <-chan posseg.Segment) []posseg.Segment {
	result := make([]posseg.Segment, 0)
	for word := range ch {
		result = append(result, word)
	}
	return result
}

func unique(src []string) []string {
	var result []string
	tempMap := map[string]byte{} // 存放不重复主键
	for _, e := range src {
		l := len(tempMap)
		tempMap[e] = 0
		if len(tempMap) != l { // 加入map后，map长度变化，则元素不重复
			result = append(result, e)
		}
	}
	return result
}

func extractInfo(filename, text string, config map[string]*regexp.Regexp) map[string]string {
	result := map[string]string{}
	defer func() {
		if p := recover(); p != nil {
			fmt.Printf("panic recover! p: %v", p)
			debug.PrintStack()
		}
	}()

	for field, exp := range config {
		if exp == nil {
			continue
		}
		var allString []string
		switch field {
		case "name":
			base := filepath.Base(filename)
			name := regexp.MustCompile("([^\\s]+)的简历")
			result[field] = first(name.FindAllString(base, -1))
			break
		case "school":
			allString = exp.FindAllString(text, -1)
			if allString != nil {
				segments := toArray(jieba.Cut(strings.Join(allString, " "), true))
				schoolList := make([]string, 0)
				schoolNames := make(map[string]string)

				for _, seg := range segments {
					if seg.Pos() == "ntu" {
						if _, OK := schoolNames[seg.Text()]; !OK {
							schoolNames[seg.Text()] = seg.Pos()
							schoolList = append(schoolList, seg.Text())
						}
					}
				}

				if len(schoolList) != 0 {
					result[field] = strings.Join(schoolList, ",")
				} else {
					result[field] = strings.Join(unique(allString), ",")
				}
			}
			break
		case "phone":
			split := regexp.MustCompile("[ -]+")
			result[field] = split.ReplaceAllString(exp.FindString(text), "")
			break
		default:
			allString = exp.FindAllString(text, -1)
			result[field] = strings.Join(allString, ",")
			break
		}
	}
	return result
}
