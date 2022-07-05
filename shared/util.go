package shared

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"regexp"
	"strings"
)

//Contains method using reflect
func Contains(list interface{}, elem interface{}) bool {
	if list == nil {
		return false
	}
	v := reflect.ValueOf(list)
	for i := 0; i < v.Len(); i++ {
		if v.Index(i).Interface() == elem {
			return true
		}
	}
	return false
}

func Any[T any, A ~[]T](slice A, predicate func(T) bool) bool {
	for _, v := range slice {
		if predicate(v) {
			return true
		}
	}
	return false
}

func Map[T any, R any](slice []T, mapper func(T) R) []R {
	var res []R
	for _, v := range slice {
		res = append(res, mapper(v))
	}
	return res
}

func Filter[T any, A ~[]T](slice A, predicate func(T) bool) []T {
	var res []T
	for _, v := range slice {
		if predicate(v) {
			res = append(res, v)
		}
	}
	return res
}

func ReadFile(fileLocation string) []byte {
	file, err := ioutil.ReadFile(fileLocation)
	CheckErr(err, fmt.Sprintf("unable to read file content from %v, Error: %v", fileLocation, err))
	return file
}

func IsGoogleObjectURI(uri string) bool {
	return strings.HasPrefix(uri, "gs://")
}

func IsAWSObjectURI(uri string) bool {
	return strings.HasPrefix(uri, "s3://")
}

func ParseStorageObjectURI(uri string) (string, string) {
	pattern := regexp.MustCompile("([\\w-]+)\\/([\\S-]+)")
	var captureGroups []string

	if strings.HasPrefix(uri, "gs://") {
		captureGroups = pattern.FindStringSubmatch(uri[len("gs://"):])
	} else if strings.HasPrefix(uri, "s3://") {
		captureGroups = pattern.FindStringSubmatch(uri[len("s3://"):])
	}

	if len(captureGroups) < 3 {
		return "", ""
	}
	return captureGroups[1], captureGroups[2]
}

func CheckErr(err interface{}, msg interface{}) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", msg)
		os.Exit(1)
	}
}

func Log(provider ProviderName, msg string) {
	log.Println(fmt.Sprintf("%v:", provider), msg)
}
