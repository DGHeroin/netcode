package utils

import (
    "os"
    "strconv"
)

func EnvGetInt(key string, defaultValue int) int {
    str := os.Getenv(key)
    if str == "" {
        return defaultValue
    }
    val, err := strconv.Atoi(str)
    if err != nil {
        return defaultValue
    }
    return val
}
