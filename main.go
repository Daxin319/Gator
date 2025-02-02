package gator

import (
	"fmt"

	config "github.com/Daxin319/Gator/internal/config"
)

func main() {
	configFile := config.Read()

	configFile.SetUser("Lyle")

	configFile = config.Read()

	fmt.Println(configFile)
}
