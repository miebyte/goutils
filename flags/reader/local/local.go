package localreader

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/miebyte/goutils/flags/reader"
)

type localConfigReader struct {
}

func NewLocalConfigReader() *localConfigReader {
	return &localConfigReader{}
}

func (lr *localConfigReader) fileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func (lr *localConfigReader) ReadConfig(v *viper.Viper, opt *reader.Option) error {
	if opt.ConfigPath == "" || !lr.fileExists(opt.ConfigPath) {
		return nil
	}

	v.SetConfigFile(opt.ConfigPath)
	if err := v.ReadInConfig(); err != nil {
		return errors.Wrap(err, "readInConfig")
	}
	fmt.Printf("Read local config success. Config=%s\n", opt.ConfigPath)
	return nil
}
