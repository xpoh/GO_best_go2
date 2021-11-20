package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"sync"
)

const version = "1.0.0"

type app struct {
	version  string
	log      *zap.Logger
	confirm  bool
	dir      string
	del      bool
	help     bool
	loglevel string
	atom     zap.AtomicLevel
	mu       sync.RWMutex
}

// fileInfoWithPath структура для хранения информации об файле
type fileInfoWithPath struct {
	fs   fs.FileInfo
	path string
}

func (f *fileInfoWithPath) GetFileHash() (string, error) {
	err := error(nil)
	var result []byte
	var tmp [16]byte

	data, err := os.ReadFile(f.path)
	if err == nil {
		tmp = md5.Sum(data)
	}
	for i := 0; i < 16; i++ {
		result = append(result, tmp[i])
	}
	return string(result), err
}

// getFileList возвращает слайс с информацией о файлах
// Входные параметры: path - директория для поиска файлов
// Выходные параметры: list - слайс файлов, err - ошибка
func (a *app) getFileList(path string) (list []fileInfoWithPath, err error) {
	a.log.Debug("Read dir", zap.String("dir", path))

	var result []fileInfoWithPath
	var tmp = fileInfoWithPath{
		fs:   nil,
		path: "",
	}

	fileStat, err := ioutil.ReadDir(path)

	if err != nil {
		a.log.Error("Error read dir", zap.String("dir", path), zap.Error(err))
		return nil, err
	}

	a.log.Debug("Read info", zap.Int("count", len(fileStat)))

	for i := 0; i < len(fileStat); i++ {
		if fileStat[i].IsDir() {
			tmp, err := a.getFileList(path + "/" + fileStat[i].Name())
			if err == nil {
				for j := 0; j < len(tmp); j++ {
					result = append(result, tmp[j])
					a.log.Debug("Files added", zap.Int("count", len(tmp)))
				}
			} else {
				return nil, err
			}
		} else {
			tmp.path = path + "/" + fileStat[i].Name()
			tmp.fs = fileStat[i]
			result = append(result, tmp)
			a.log.Debug("Append file ", zap.String("filename", fileStat[i].Name()), zap.String("fullPath", tmp.path))
		}
	}
	return result, nil
}

// removeFile метод принимающий структуру *fileInfoWithPath
// sync.WaitGroup для синхронизации, возвращающий err - ошибку
func (f *fileInfoWithPath) removeFile(wg *sync.WaitGroup) (err error) {
	defer wg.Done()

	err = os.Remove(f.path)
	if err == nil {
		fmt.Println("Удален файл: ", f.path)
	}
	return err
}

func (a *app) InitApp() error {
	var err = error(nil)

	a.version = version
	a.ReadConfigFromFlags()
	err = a.InitLogger()
	return err
}
func (a *app) ReadConfigFromFlags() {
	dir := flag.String("p", "", "путь для поиска файлов")
	del := flag.Bool("f", false, "удаление с подтверждением повторяющихся файлов")
	help := flag.Bool("h", false, "текущая справка")
	loglevel := flag.String("l", "ERROR", "log level")
	flag.Parse()

	a.confirm = false
	a.dir = *dir
	a.del = *del
	a.help = *help
	a.loglevel = *loglevel
}

func (a *app) InitLogger() error {
	err := error(nil)
	var ll zapcore.Level
	ll.Set(a.loglevel)
	a.atom = zap.NewAtomicLevelAt(ll)
	a.log, err = zap.NewDevelopment()
	if err == nil {
		a.log.Debug("Set log level", zap.String("log_level", a.loglevel))
	}
	return err
}
func (a *app) CloseApp() error {
	a.log.Info("App close")
	err := a.log.Sync()

	return err
}

// printHelp выводит справку
func (a *app) printHelp() {
	fmt.Println("Программа удаления дублированных файлов.")
	fmt.Println("аргументы:")
	fmt.Println("-h 	текущая справка")
	fmt.Println("-p 	путь для поиска файлов")
	fmt.Println("-f 	удаление с подтверждением повторяющихся файлов")
	fmt.Println("-l 	уровень логирования. По умолчанию ERROR")
}

func main() {
	a := app{}
	err := a.InitApp()
	if err != nil {
		log.Panic(err)
		panic("Error init dup_cleaner")
	}
	defer a.CloseApp()

	a.log.Debug("Count args parsed", zap.Int("nargs", flag.NArg()))
	if flag.NFlag() == 0 {
		a.printHelp()
		return
	}

	a.log.Info("Successful init app.", zap.String("version", version))

	wg := sync.WaitGroup{}

	if a.help {
		a.printHelp()
		a.log.Info("Help printed")
		return
	}

	if a.del {
		fmt.Print("Удалять файлы сразу? [напиши YES] ")
		var s string
		_, _ = fmt.Scanln(&s)

		if s == "YES" {
			a.confirm = true
		}
		a.log.Debug("Confirm delete", zap.Bool("confirm", a.confirm))
	}

	list, err := a.getFileList(a.dir)
	if err != nil {
		a.log.Info("Error get file list", zap.Error(err))
		return
	}

	fmt.Println("Найдено файлов: ", len(list))
	a.log.Info("Files found ", zap.Int("foundcount", len(list)))
	m := make(map[string]string)

	for i := 0; i < len(list); i++ {
		hash, err := list[i].GetFileHash()
		if err != nil {
			a.log.Error("Error calc hash", zap.Int("index", i), zap.String("filename", list[i].path), zap.Error(err))
		}
		a.log.Debug("file md5 hash", zap.String("filename", list[i].path), zap.String("md5", hash))

		_, ok := m[hash]
		if !ok {
			m[hash] = list[i].path
			fmt.Println("Uniq file: ", list[i].path)
			a.log.Info("New uniq file", zap.String("filename", list[i].path))
		} else {
			a.log.Info("New duplicate file", zap.String("filename", list[i].path))
			if a.confirm {
				name := list[i]
				wg.Add(1)
				go func() {
					err = name.removeFile(&wg)
					if err != nil {
						a.log.Error("Error delete file", zap.String("filename", name.path), zap.Error(err))
					} else {
						a.log.Info("File deleted", zap.String("filename", name.path))
					}
				}()
			}
		}
	}
	a.log.Info("Waiting to complete...")
	wg.Wait()
}
