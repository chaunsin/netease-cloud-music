// MIT License
//
// Copyright (c) 2024 chaunsin
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//

package ncmctl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chaunsin/netease-cloud-music/pkg/utils"

	"github.com/spf13/cobra"
)

func writeFile(cmd *cobra.Command, out string, data []byte) error {
	if out == "" {
		cmd.Println(string(data))
		return nil
	}

	// 写入文件
	var file string
	if !filepath.IsAbs(out) {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		file = filepath.Join(wd, out)
		if !utils.PathExists(file) {
			if err := os.MkdirAll(filepath.Dir(file), os.ModePerm); err != nil {
				return fmt.Errorf("MkdirAll: %w", err)
			}
		}
	}
	if err := os.WriteFile(file, data, os.ModePerm); err != nil {
		return fmt.Errorf("WriteFile: %w", err)
	}
	cmd.Printf("generate file path: %s\n", file)
	return nil
}

func parseArgs(args []string) (map[string]map[string]string, error) {
	var argMap = make(map[string]map[string]string)
	for i := 0; i < len(args); i++ {
		var arg = args[i]

		// 检查参数是否以"--"开头
		if !strings.HasPrefix(arg, "--") {
			return nil, fmt.Errorf("unexpected argument format: %s", arg)
		}

		// 去掉"--"前缀，获取参数部分
		arg = strings.TrimPrefix(arg, "--")

		// 将参数按照第一个"."分割为命令名称和参数部分
		parts := strings.SplitN(arg, ".", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid argument format: %s", arg)
		}

		var (
			cmdName    = parts[0]                          // 提取命令名称
			paramPart  = parts[1]                          // 提取参数部分
			paramParts = strings.SplitN(paramPart, "=", 2) // 将参数部分按照"="分割为参数名称和参数值
			paramName  = paramParts[0]
			paramValue string
		)

		// 如果参数映射中没有当前命令的映射，初始化一个空映射
		if _, ok := argMap[cmdName]; !ok {
			argMap[cmdName] = make(map[string]string)
		}

		// 如果分割后的部分是两个，表示有指定的参数值
		if len(paramParts) == 2 {
			paramValue = paramParts[1]
		} else {
			// 否则，检查下一个参数是否存在，并且不以"--"开头，作为参数值
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				paramValue = args[i+1]
				i++ // 跳过下一个参数
			} else {
				// 如果没有指定参数值，默认设置为"true"
				paramValue = "true"
			}
		}
		argMap[cmdName][paramName] = paramValue
	}
	return argMap, nil
}

func setCmdArgs(cmd *cobra.Command, args map[string]string) error {
	for name, value := range args {
		flag := cmd.Flags().Lookup(name)
		if flag == nil {
			return fmt.Errorf("unknown flag: %s", name)
		}
		if err := flag.Value.Set(value); err != nil {
			return fmt.Errorf("invalid value for flag %s: %v", name, err)
		}
	}
	return nil
}
