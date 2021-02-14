package util

import "os"

func FileExists(filename string) bool {
    info, err := os.Stat(filename)
    if os.IsNotExist(err) {
        return false
    }
    return !info.IsDir()
}

func DirExists(dirname string) bool{
    info, err := os.Stat(dirname)
    if os.IsNotExist(err) {
        return false
    }
    return info.IsDir()
}

func EnsureDir(dirname string) error{
    if !DirExists(dirname) {
        return os.Mkdir(dirname, 0755)
    }
    return nil
}
