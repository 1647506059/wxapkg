package cmd

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/wux1an/wxapkg/util"
	"os"
	"path/filepath"
	"regexp"
)

var scanCmd = &cobra.Command{
	Use:     "scan",
	Short:   "Scan the wechat mini program",
	Example: "  " + programName + " scan -r \"D:\\WeChat Files\\Applet\\wx12345678901234\"",
	Run: func(cmd *cobra.Command, args []string) {
		root, err := cmd.Flags().GetString("root")
		if err != nil {
			color.Red("%v", err)
			return
		}

		var scanRoots = []string{}
		if cmd.Flags().Changed("root") {
			scanRoots = append(scanRoots, root)
		} else {
			homeDir, _ := os.UserHomeDir()
			// 1. 文档路径 (Documents/WeChat Files/Applet)
			scanRoots = append(scanRoots, filepath.Join(homeDir, "Documents", "WeChat Files", "Applet"))

			// 2. 多用户路径 (AppData/Roaming/Tencent/xwechat/radium/users/${userId}/applet/packages)
			usersDir := filepath.Join(homeDir, "AppData/Roaming/Tencent/xwechat/radium/users")
			if entries, err := os.ReadDir(usersDir); err == nil {
				for _, entry := range entries {
					if entry.IsDir() {
						scanRoots = append(scanRoots, filepath.Join(usersDir, entry.Name(), "applet", "packages"))
					}
				}
			}

			// 3. 默认路径 (AppData/Roaming/Tencent/xwechat/radium/Applet/packages)
			scanRoots = append(scanRoots, root)
		}

		var regAppId = regexp.MustCompile(`(wx[0-9a-f]{16})`)
		var wxidInfos = make([]util.WxidInfo, 0)
		var scannedPaths = make(map[string]bool)

		for _, path := range scanRoots {
			// 简单的路径去重
			if scannedPaths[path] {
				continue
			}
			scannedPaths[path] = true

			files, err := os.ReadDir(path)
			if err != nil {
				// 忽略不存在的路径或权限错误，继续扫描其他路径
				continue
			}

			var currentInfos = make([]util.WxidInfo, 0)
			for _, file := range files {
				if !file.IsDir() || !regAppId.MatchString(file.Name()) {
					continue
				}

				var wxid = regAppId.FindStringSubmatch(file.Name())[1]
				info, err := util.WxidQuery.Query(wxid)
				info.Location = filepath.Join(path, file.Name())
				info.Wxid = wxid
				if err != nil {
					info.Error = fmt.Sprintf("%v", err)
				}
				if info.Nickname == "" {
					info.Nickname = wxid
				}

				currentInfos = append(currentInfos, info)
			}

			// 如果当前路径找到了小程序，则停止扫描后续路径
			if len(currentInfos) > 0 {
				wxidInfos = currentInfos
				break
			}
		}

		var tui = newScanTui(wxidInfos)
		if _, err := tea.NewProgram(tui, tea.WithAltScreen()).Run(); err != nil {
			color.Red("Error running program: %v", err)
			os.Exit(1)
		}

		if tui.selected == nil {
			return
		}

		output := tui.selected.Wxid
		_ = unpackCmd.Flags().Set("root", tui.selected.Location)
		_ = unpackCmd.Flags().Set("output", output)
		detailFilePath := filepath.Join(output, "detail.json")
		unpackCmd.Run(unpackCmd, []string{"detailFilePath", detailFilePath})
		_ = os.WriteFile(detailFilePath, []byte(tui.selected.Json()), 0600)
	},
}

func init() {
	RootCmd.AddCommand(scanCmd)

	// Documents/WeChat Files/Applet
	// C:\Users\Administrator\AppData\Roaming\Tencent\xwechat\radium\users\${userId}\applet\packages
	// 注意：userId是微信给你的用户ID，比如：userId=2a5dcfb3931d78f2965f8818b8eb1183;
	// C:\Users\Administrator\AppData\Roaming\Tencent\xwechat\radium\Applet\packages

	var homeDir, _ = os.UserHomeDir()
	var defaultRoot = filepath.Join(homeDir, "AppData/Roaming/Tencent/xwechat/radium/Applet/packages")

	scanCmd.Flags().StringP("root", "r", defaultRoot, "the mini app path")
}
