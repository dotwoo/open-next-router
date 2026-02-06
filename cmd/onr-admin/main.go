package main

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/r9s-ai/open-next-router/internal/config"
	"github.com/r9s-ai/open-next-router/internal/keystore"
	"github.com/r9s-ai/open-next-router/internal/models"
	"github.com/r9s-ai/open-next-router/pkg/dslconfig"
	"gopkg.in/yaml.v3"
)

func main() {
	if err := runCLI(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

type app struct {
	in           *bufio.Reader
	out          io.Writer
	cfgPath      string
	keysPath     string
	modelsPath   string
	providersDir string
	backup       bool
	masterKey    string

	keysDoc     *yaml.Node
	keysDirty   bool
	modelsDoc   *yaml.Node
	modelsDirty bool
}

func (a *app) run() error {
	for {
		fmt.Fprintln(a.out, "")
		fmt.Fprintln(a.out, "+--------------------------------------------------+")
		fmt.Fprintln(a.out, "| ONR Admin TUI                                    |")
		fmt.Fprintln(a.out, "+--------------------------------------------------+")
		fmt.Fprintf(a.out, " config       : %s\n", strings.TrimSpace(a.cfgPath))
		fmt.Fprintf(a.out, " keys.yaml    : %s\n", strings.TrimSpace(a.keysPath))
		fmt.Fprintf(a.out, " models.yaml  : %s\n", strings.TrimSpace(a.modelsPath))
		fmt.Fprintf(a.out, " providers dir: %s\n", strings.TrimSpace(a.providersDir))
		fmt.Fprintf(a.out, " dirty        : keys=%v models=%v\n", a.keysDirty, a.modelsDirty)
		fmt.Fprintln(a.out, "")
		fmt.Fprintln(a.out, " [k] keys.yaml 管理")
		fmt.Fprintln(a.out, " [m] models.yaml 管理")
		fmt.Fprintln(a.out, " [t] 生成 Token Key (onr:v1?)")
		fmt.Fprintln(a.out, " [v] 校验/诊断")
		fmt.Fprintln(a.out, " [s] 保存所有脏文件")
		fmt.Fprintln(a.out, " [q] 退出")

		choice, err := a.readMenuChoice("选择(k/m/t/v/s/q): ", []string{"k", "m", "t", "v", "s", "q"})
		if err != nil {
			return err
		}
		switch choice {
		case "k":
			if err := a.menuKeys(); err != nil {
				return err
			}
		case "m":
			if err := a.menuModels(); err != nil {
				return err
			}
		case "t":
			if err := a.menuGenKey(); err != nil {
				return err
			}
		case "v":
			if err := a.menuValidate(); err != nil {
				return err
			}
		case "s":
			if err := a.saveAll(); err != nil {
				return err
			}
		case "q":
			if a.keysDirty || a.modelsDirty {
				ok, err := a.confirm("有未保存的更改，确认退出？(y/N): ")
				if err != nil {
					return err
				}
				if !ok {
					continue
				}
			}
			return nil
		}
	}
}

func (a *app) menuKeys() error {
	for {
		fmt.Fprintln(a.out, "")
		fmt.Fprintln(a.out, "== keys.yaml ==")
		fmt.Fprintln(a.out, "1) providers (上游 key 池)")
		fmt.Fprintln(a.out, "2) access_keys (访问 key 池)")
		fmt.Fprintln(a.out, "3) 返回")

		choice, err := a.readChoice(1, 3)
		if err != nil {
			return err
		}
		switch choice {
		case 1:
			if err := a.menuProviders(); err != nil {
				return err
			}
		case 2:
			if err := a.menuAccessKeys(); err != nil {
				return err
			}
		case 3:
			return nil
		}
	}
}

func (a *app) menuModels() error {
	for {
		ids := listModelIDs(a.modelsDoc)
		fmt.Fprintln(a.out, "")
		fmt.Fprintln(a.out, "== models.yaml ==")
		if len(ids) == 0 {
			fmt.Fprintln(a.out, "(没有 model)")
		} else {
			for i, id := range ids {
				fmt.Fprintf(a.out, "%d) %s\n", i+1, id)
			}
		}
		fmt.Fprintln(a.out, "")
		fmt.Fprintln(a.out, "a) 新增 model")
		fmt.Fprintln(a.out, "e) 编辑 model")
		fmt.Fprintln(a.out, "d) 删除 model")
		fmt.Fprintln(a.out, "b) 返回")

		s, err := a.readLine("选择(数字) 或命令(a/e/d/b): ")
		if err != nil {
			return err
		}
		s = strings.ToLower(strings.TrimSpace(s))
		switch s {
		case "b":
			return nil
		case "a":
			if err := a.addModel(); err != nil {
				return err
			}
		case "e":
			if err := a.editModel(ids); err != nil {
				return err
			}
		case "d":
			if err := a.deleteModelEntry(ids); err != nil {
				return err
			}
		default:
			a.printModelByIndex(s, ids)
		}
	}
}

func (a *app) addModel() error {
	id, err := a.readLine("model id: ")
	if err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	if _, ok := getModelNode(a.modelsDoc, id); ok {
		fmt.Fprintln(a.out, "已存在该 model。")
		return nil
	}

	provsIn, err := a.readLine("providers(逗号分隔，如 openai,anthropic): ")
	if err != nil {
		return err
	}
	provs := parseProviders(provsIn)

	strategy, err := a.readLine("strategy(默认 round_robin): ")
	if err != nil {
		return err
	}
	strategy = strings.TrimSpace(strategy)
	if strategy == "" {
		strategy = string(models.StrategyRoundRobin)
	}

	ownedBy, err := a.readLine("owned_by(可空): ")
	if err != nil {
		return err
	}

	rt := models.Route{
		Providers: provs,
		Strategy:  models.Strategy(strings.TrimSpace(strategy)),
		OwnedBy:   strings.TrimSpace(ownedBy),
	}
	if err := setModelRoute(a.modelsDoc, id, rt); err != nil {
		fmt.Fprintf(a.out, "新增失败: %v\n", err)
		return nil
	}
	a.modelsDirty = true
	return nil
}

func (a *app) editModel(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	n, err := a.readInt("输入要编辑的 model 序号: ")
	if err != nil {
		return err
	}
	if n <= 0 || n > len(ids) {
		return nil
	}

	id := ids[n-1]
	rt, _ := getModelRoute(a.modelsDoc, id)
	fmt.Fprintf(a.out, "当前 providers=%q strategy=%q owned_by=%q\n", strings.Join(rt.Providers, ","), string(rt.Strategy), rt.OwnedBy)

	provsIn, err := a.readLine("新 providers(留空=不改，输入 '-'=清空): ")
	if err != nil {
		return err
	}
	strategy, err := a.readLine("新 strategy(留空=不改，输入 '-'=清空): ")
	if err != nil {
		return err
	}
	ownedBy, err := a.readLine("新 owned_by(留空=不改，输入 '-'=清空): ")
	if err != nil {
		return err
	}

	up := modelUpdate{}
	if strings.TrimSpace(provsIn) != "" {
		if strings.TrimSpace(provsIn) == "-" {
			up.Providers = ptr([]string(nil))
		} else {
			up.Providers = ptr(parseProviders(provsIn))
		}
	}
	if strings.TrimSpace(strategy) != "" {
		if strings.TrimSpace(strategy) == "-" {
			up.Strategy = ptr("")
		} else {
			up.Strategy = ptr(strings.TrimSpace(strategy))
		}
	}
	if strings.TrimSpace(ownedBy) != "" {
		if strings.TrimSpace(ownedBy) == "-" {
			up.OwnedBy = ptr("")
		} else {
			up.OwnedBy = ptr(strings.TrimSpace(ownedBy))
		}
	}

	if err := updateModelRoute(a.modelsDoc, id, up); err != nil {
		fmt.Fprintf(a.out, "编辑失败: %v\n", err)
		return nil
	}
	a.modelsDirty = true
	return nil
}

func (a *app) deleteModelEntry(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	name, err := a.readLine("输入要删除的 model id: ")
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	ok, err := a.confirm("确认删除 model " + name + " ? (y/N): ")
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if err := deleteModel(a.modelsDoc, name); err != nil {
		fmt.Fprintf(a.out, "删除失败: %v\n", err)
		return nil
	}
	a.modelsDirty = true
	return nil
}

func (a *app) printModelByIndex(s string, ids []string) {
	// allow quick enter by index
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 || n > len(ids) {
		return
	}
	id := ids[n-1]
	rt, _ := getModelRoute(a.modelsDoc, id)
	fmt.Fprintln(a.out, "")
	fmt.Fprintf(a.out, "model=%s providers=%q strategy=%q owned_by=%q\n", id, strings.Join(rt.Providers, ","), string(rt.Strategy), rt.OwnedBy)
}

func (a *app) menuProviders() error {
	for {
		provs := listProviders(a.keysDoc)
		fmt.Fprintln(a.out, "")
		fmt.Fprintln(a.out, "== providers (上游 key 池) ==")
		if len(provs) == 0 {
			fmt.Fprintln(a.out, "(没有 provider)")
		} else {
			for i, p := range provs {
				fmt.Fprintf(a.out, "%d) %s\n", i+1, p)
			}
		}
		fmt.Fprintln(a.out, "")
		fmt.Fprintln(a.out, "a) 新增 provider")
		fmt.Fprintln(a.out, "d) 删除 provider")
		fmt.Fprintln(a.out, "b) 返回")

		s, err := a.readLine("选择 provider(数字) 或命令(a/d/b): ")
		if err != nil {
			return err
		}
		s = strings.ToLower(strings.TrimSpace(s))
		switch s {
		case "b":
			return nil
		case "a":
			name, err := a.readLine("输入 provider 名称: ")
			if err != nil {
				return err
			}
			name = strings.ToLower(strings.TrimSpace(name))
			if name == "" {
				continue
			}
			if _, ok := getProviderNode(a.keysDoc, name); ok {
				fmt.Fprintln(a.out, "已存在该 provider。")
				continue
			}
			ensureProviderNode(a.keysDoc, name)
			a.keysDirty = true
		case "d":
			if len(provs) == 0 {
				continue
			}
			name, err := a.readLine("输入要删除的 provider 名称: ")
			if err != nil {
				return err
			}
			name = strings.ToLower(strings.TrimSpace(name))
			if name == "" {
				continue
			}
			ok, err := a.confirm("确认删除 provider " + name + " ? (y/N): ")
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			if err := deleteProvider(a.keysDoc, name); err != nil {
				fmt.Fprintf(a.out, "删除失败: %v\n", err)
				continue
			}
			a.keysDirty = true
		default:
			n, err := strconv.Atoi(s)
			if err != nil || n <= 0 || n > len(provs) {
				continue
			}
			p := provs[n-1]
			if err := a.menuProvider(p); err != nil {
				return err
			}
		}
	}
}

func (a *app) menuAccessKeys() error {
	for {
		aks, _ := listAccessKeysDoc(a.keysDoc)
		fmt.Fprintln(a.out, "")
		fmt.Fprintln(a.out, "== access_keys (访问 key 池) ==")
		if len(aks) == 0 {
			fmt.Fprintln(a.out, "(没有 access_key)")
		} else {
			for i, ak := range aks {
				name := strings.TrimSpace(ak.Name)
				if name == "" {
					name = fmt.Sprintf("#%d", i+1)
				}
				disabled := ""
				if ak.Disabled {
					disabled = " disabled=true"
				}
				c := strings.TrimSpace(ak.Comment)
				if c != "" {
					c = " comment=" + strconv.Quote(c)
				}
				fmt.Fprintf(a.out, "%d) name=%q value=%s%s%s\n", i+1, name, valueHint(ak.Value), disabled, c)
			}
		}
		fmt.Fprintln(a.out, "")
		fmt.Fprintln(a.out, "1) 新增 access_key")
		fmt.Fprintln(a.out, "2) 编辑 access_key")
		fmt.Fprintln(a.out, "3) 删除 access_key")
		fmt.Fprintln(a.out, "4) 环境变量覆盖提示")
		fmt.Fprintln(a.out, "5) 返回")

		choice, err := a.readChoice(1, 5)
		if err != nil {
			return err
		}
		switch choice {
		case 1:
			if err := a.addAccessKey(); err != nil {
				return err
			}
		case 2:
			if err := a.editAccessKey(); err != nil {
				return err
			}
		case 3:
			if err := a.deleteAccessKey(); err != nil {
				return err
			}
		case 4:
			a.printAccessKeyEnvHints()
		case 5:
			return nil
		}
	}
}

func (a *app) addAccessKey() error {
	name, err := a.readLine("name(建议填写): ")
	if err != nil {
		return err
	}
	val, err := a.readLine("value(可空；支持明文或 ENC[...]): ")
	if err != nil {
		return err
	}
	disabledStr, err := a.readLine("disabled(y/N): ")
	if err != nil {
		return err
	}
	comment, err := a.readLine("comment(可空): ")
	if err != nil {
		return err
	}

	disabled := false
	switch strings.ToLower(strings.TrimSpace(disabledStr)) {
	case "y", "yes", "true", "1":
		disabled = true
	}

	val = strings.TrimSpace(val)
	if val != "" {
		enc, err := a.confirm("要把 value 加密写入 keys.yaml 吗？(y/N): ")
		if err != nil {
			return err
		}
		if enc {
			out, err := keystore.Encrypt(val)
			if err != nil {
				fmt.Fprintf(a.out, "加密失败: %v\n", err)
				return nil
			}
			val = out
		}
	}

	if err := appendAccessKeyDoc(a.keysDoc, keystore.AccessKey{
		Name:     strings.TrimSpace(name),
		Value:    val,
		Disabled: disabled,
		Comment:  strings.TrimSpace(comment),
	}); err != nil {
		fmt.Fprintf(a.out, "新增失败: %v\n", err)
		return nil
	}
	a.keysDirty = true
	return nil
}

func (a *app) editAccessKey() error {
	aks, _ := listAccessKeysDoc(a.keysDoc)
	if len(aks) == 0 {
		return nil
	}
	idx, err := a.readInt("输入要编辑的 access_key 序号: ")
	if err != nil {
		return err
	}
	if idx <= 0 || idx > len(aks) {
		return nil
	}
	cur := aks[idx-1]
	fmt.Fprintf(a.out, "当前 name=%q value=%s disabled=%v comment=%q\n", cur.Name, valueHint(cur.Value), cur.Disabled, cur.Comment)

	name, err := a.readLine("新 name(留空=不改，输入 '-'=清空): ")
	if err != nil {
		return err
	}
	val, err := a.readLine("新 value(留空=不改，输入 '-'=清空): ")
	if err != nil {
		return err
	}
	disabledStr, err := a.readLine("新 disabled(y/N；留空=不改): ")
	if err != nil {
		return err
	}
	comment, err := a.readLine("新 comment(留空=不改，输入 '-'=清空): ")
	if err != nil {
		return err
	}

	up := accessKeyUpdate{}
	if strings.TrimSpace(name) != "" {
		if strings.TrimSpace(name) == "-" {
			up.Name = ptr("")
		} else {
			up.Name = ptr(strings.TrimSpace(name))
		}
	}
	if strings.TrimSpace(val) != "" {
		if strings.TrimSpace(val) == "-" {
			up.Value = ptr("")
		} else {
			v := strings.TrimSpace(val)
			enc, err := a.confirm("要把 value 加密写入 keys.yaml 吗？(y/N): ")
			if err != nil {
				return err
			}
			if enc && v != "" {
				out, err := keystore.Encrypt(v)
				if err != nil {
					fmt.Fprintf(a.out, "加密失败: %v\n", err)
					return nil
				}
				v = out
			}
			up.Value = ptr(v)
		}
	}
	if strings.TrimSpace(disabledStr) != "" {
		switch strings.ToLower(strings.TrimSpace(disabledStr)) {
		case "y", "yes", "true", "1":
			up.Disabled = ptr(true)
		default:
			up.Disabled = ptr(false)
		}
	}
	if strings.TrimSpace(comment) != "" {
		if strings.TrimSpace(comment) == "-" {
			up.Comment = ptr("")
		} else {
			up.Comment = ptr(strings.TrimSpace(comment))
		}
	}

	if err := updateAccessKeyDoc(a.keysDoc, idx-1, up); err != nil {
		fmt.Fprintf(a.out, "编辑失败: %v\n", err)
		return nil
	}
	a.keysDirty = true
	return nil
}

func (a *app) deleteAccessKey() error {
	aks, _ := listAccessKeysDoc(a.keysDoc)
	if len(aks) == 0 {
		return nil
	}
	idx, err := a.readInt("输入要删除的 access_key 序号: ")
	if err != nil {
		return err
	}
	if idx <= 0 || idx > len(aks) {
		return nil
	}
	ok, err := a.confirm("确认删除该 access_key ? (y/N): ")
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if err := deleteAccessKeyDoc(a.keysDoc, idx-1); err != nil {
		fmt.Fprintf(a.out, "删除失败: %v\n", err)
		return nil
	}
	a.keysDirty = true
	return nil
}

func (a *app) printAccessKeyEnvHints() {
	aks, _ := listAccessKeysDoc(a.keysDoc)
	fmt.Fprintln(a.out, "")
	fmt.Fprintln(a.out, "== env 覆盖提示: access_keys ==")
	if len(aks) == 0 {
		fmt.Fprintln(a.out, "(没有 access_key)")
		return
	}
	for i, ak := range aks {
		fmt.Fprintf(a.out, "%d) %s\n", i+1, accessEnvVar(strings.TrimSpace(ak.Name), i))
	}
}

func (a *app) menuProvider(provider string) error {
	for {
		keys, _ := listProviderKeys(a.keysDoc, provider)
		fmt.Fprintln(a.out, "")
		fmt.Fprintf(a.out, "== provider: %s ==\n", provider)
		if len(keys) == 0 {
			fmt.Fprintln(a.out, "(没有 key)")
		} else {
			for i, k := range keys {
				valHint := valueHint(k.Value)
				bu := strings.TrimSpace(k.BaseURLOverride)
				if bu != "" {
					bu = " base_url_override=" + bu
				}
				fmt.Fprintf(a.out, "%d) name=%q value=%s%s\n", i+1, strings.TrimSpace(k.Name), valHint, bu)
			}
		}
		fmt.Fprintln(a.out, "")
		fmt.Fprintln(a.out, "1) 新增 key")
		fmt.Fprintln(a.out, "2) 编辑 key")
		fmt.Fprintln(a.out, "3) 删除 key")
		fmt.Fprintln(a.out, "4) 环境变量覆盖提示")
		fmt.Fprintln(a.out, "5) 返回")

		choice, err := a.readChoice(1, 5)
		if err != nil {
			return err
		}
		switch choice {
		case 1:
			if err := a.addKey(provider); err != nil {
				return err
			}
		case 2:
			if err := a.editKey(provider); err != nil {
				return err
			}
		case 3:
			if err := a.deleteKey(provider); err != nil {
				return err
			}
		case 4:
			a.printEnvHints(provider)
		case 5:
			return nil
		}
	}
}

func (a *app) addKey(provider string) error {
	name, err := a.readLine("name(可空): ")
	if err != nil {
		return err
	}
	val, err := a.readLine("value(可空；支持明文或 ENC[...]): ")
	if err != nil {
		return err
	}
	bu, err := a.readLine("base_url_override(可空): ")
	if err != nil {
		return err
	}

	val = strings.TrimSpace(val)
	if val != "" {
		enc, err := a.confirm("要把 value 加密写入 keys.yaml 吗？(y/N): ")
		if err != nil {
			return err
		}
		if enc {
			out, err := keystore.Encrypt(val)
			if err != nil {
				fmt.Fprintf(a.out, "加密失败: %v\n", err)
				return nil
			}
			val = out
		}
	}

	if err := appendProviderKey(a.keysDoc, provider, keystore.Key{
		Name:            strings.TrimSpace(name),
		Value:           val,
		BaseURLOverride: strings.TrimSpace(bu),
	}); err != nil {
		fmt.Fprintf(a.out, "新增失败: %v\n", err)
		return nil
	}
	a.keysDirty = true
	return nil
}

func (a *app) editKey(provider string) error {
	keys, _ := listProviderKeys(a.keysDoc, provider)
	if len(keys) == 0 {
		return nil
	}
	idx, err := a.readInt("输入要编辑的 key 序号: ")
	if err != nil {
		return err
	}
	if idx <= 0 || idx > len(keys) {
		return nil
	}
	cur := keys[idx-1]
	fmt.Fprintf(a.out, "当前 name=%q value=%s base_url_override=%q\n", cur.Name, valueHint(cur.Value), cur.BaseURLOverride)

	name, err := a.readLine("新 name(留空=不改，输入 '-'=清空): ")
	if err != nil {
		return err
	}
	val, err := a.readLine("新 value(留空=不改，输入 '-'=清空): ")
	if err != nil {
		return err
	}
	bu, err := a.readLine("新 base_url_override(留空=不改，输入 '-'=清空): ")
	if err != nil {
		return err
	}

	update := keyUpdate{}
	if strings.TrimSpace(name) != "" {
		if strings.TrimSpace(name) == "-" {
			update.Name = ptr("")
		} else {
			update.Name = ptr(strings.TrimSpace(name))
		}
	}
	if strings.TrimSpace(val) != "" {
		if strings.TrimSpace(val) == "-" {
			update.Value = ptr("")
		} else {
			v := strings.TrimSpace(val)
			enc, err := a.confirm("要把 value 加密写入 keys.yaml 吗？(y/N): ")
			if err != nil {
				return err
			}
			if enc && v != "" {
				out, err := keystore.Encrypt(v)
				if err != nil {
					fmt.Fprintf(a.out, "加密失败: %v\n", err)
					return nil
				}
				v = out
			}
			update.Value = ptr(v)
		}
	}
	if strings.TrimSpace(bu) != "" {
		if strings.TrimSpace(bu) == "-" {
			update.BaseURLOverride = ptr("")
		} else {
			update.BaseURLOverride = ptr(strings.TrimSpace(bu))
		}
	}

	if err := updateProviderKey(a.keysDoc, provider, idx-1, update); err != nil {
		fmt.Fprintf(a.out, "编辑失败: %v\n", err)
		return nil
	}
	a.keysDirty = true
	return nil
}

func (a *app) deleteKey(provider string) error {
	keys, _ := listProviderKeys(a.keysDoc, provider)
	if len(keys) == 0 {
		return nil
	}
	idx, err := a.readInt("输入要删除的 key 序号: ")
	if err != nil {
		return err
	}
	if idx <= 0 || idx > len(keys) {
		return nil
	}
	ok, err := a.confirm("确认删除该 key ? (y/N): ")
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if err := deleteProviderKey(a.keysDoc, provider, idx-1); err != nil {
		fmt.Fprintf(a.out, "删除失败: %v\n", err)
		return nil
	}
	a.keysDirty = true
	return nil
}

func (a *app) printEnvHints(provider string) {
	keys, _ := listProviderKeys(a.keysDoc, provider)
	fmt.Fprintln(a.out, "")
	fmt.Fprintf(a.out, "== env 覆盖提示: provider=%s ==\n", provider)
	if len(keys) == 0 {
		fmt.Fprintln(a.out, "(没有 key)")
		return
	}
	for i, k := range keys {
		fmt.Fprintf(a.out, "%d) %s\n", i+1, upstreamEnvVar(provider, strings.TrimSpace(k.Name), i))
	}
}

func (a *app) menuGenKey() error {
	fmt.Fprintln(a.out, "")
	fmt.Fprintln(a.out, "== 生成 Token Key (onr:v1?) ==")
	accessKey, err := a.pickAccessKey()
	if err != nil {
		return err
	}

	p, err := a.readLine("provider p(可空): ")
	if err != nil {
		return err
	}
	m, err := a.readLine("model override m(可空): ")
	if err != nil {
		return err
	}
	uk, err := a.readLine("BYOK upstream key uk(可空): ")
	if err != nil {
		return err
	}

	vals := url.Values{}
	vals.Set("k64", base64.RawURLEncoding.EncodeToString([]byte(accessKey)))
	if strings.TrimSpace(p) != "" {
		vals.Set("p", strings.ToLower(strings.TrimSpace(p)))
	}
	if strings.TrimSpace(m) != "" {
		vals.Set("m", strings.TrimSpace(m))
	}
	if strings.TrimSpace(uk) != "" {
		vals.Set("uk", strings.TrimSpace(uk))
	}

	key := "onr:v1?" + vals.Encode()
	fmt.Fprintln(a.out, "")
	fmt.Fprintln(a.out, "生成结果：")
	fmt.Fprintln(a.out, key)
	return nil
}

func (a *app) pickAccessKey() (string, error) {
	ks, err := keystore.Load(a.keysPath)
	if err != nil {
		fmt.Fprintf(a.out, "提示：无法加载 keys.yaml 解析 access_keys（可能缺少 ONR_MASTER_KEY 以解密 ENC[...]）：%v\n", err)
	}
	aks := []keystore.AccessKey{}
	if ks != nil {
		aks = ks.AccessKeys()
	}

	fmt.Fprintln(a.out, "选择 access_key:")
	if len(aks) > 0 {
		for i, ak := range aks {
			name := strings.TrimSpace(ak.Name)
			if name == "" {
				name = fmt.Sprintf("#%d", i+1)
			}
			c := strings.TrimSpace(ak.Comment)
			if c != "" {
				c = " (" + c + ")"
			}
			fmt.Fprintf(a.out, "%d) %s%s\n", i+1, name, c)
		}
	}
	fmt.Fprintln(a.out, "m) 手动输入 access_key")

	s, err := a.readLine("选择(数字/m): ")
	if err != nil {
		return "", err
	}
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "m" || s == "" {
		def := strings.TrimSpace(a.masterKey)
		if def != "" {
			fmt.Fprintln(a.out, "提示：默认使用 auth.api_key 作为 access_key（仅用于演示；推荐在 keys.yaml 配置 access_keys）。")
		}
		in, err := a.readLine("access_key(留空=使用默认): ")
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(in) != "" {
			def = strings.TrimSpace(in)
		}
		if def == "" {
			return "", errors.New("missing access_key")
		}
		return def, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 || n > len(aks) {
		return "", errors.New("invalid selection")
	}
	return strings.TrimSpace(aks[n-1].Value), nil
}

func (a *app) menuValidate() error {
	fmt.Fprintln(a.out, "")
	fmt.Fprintln(a.out, "== 校验/诊断 ==")
	if err := validateKeysDoc(a.keysDoc); err != nil {
		fmt.Fprintf(a.out, "keys.yaml(结构) 校验失败: %v\n", err)
	} else {
		fmt.Fprintln(a.out, "keys.yaml(结构) 校验: OK")
	}
	if _, err := keystore.Load(a.keysPath); err != nil {
		fmt.Fprintf(a.out, "keystore.Load(%s) 失败: %v\n", a.keysPath, err)
	} else {
		fmt.Fprintf(a.out, "keystore.Load(%s): OK\n", a.keysPath)
	}
	if err := validateModelsDoc(a.modelsDoc); err != nil {
		fmt.Fprintf(a.out, "models.yaml(结构) 校验失败: %v\n", err)
	} else {
		fmt.Fprintln(a.out, "models.yaml(结构) 校验: OK")
	}
	if _, err := models.Load(a.modelsPath); err != nil {
		fmt.Fprintf(a.out, "models.Load(%s) 失败: %v\n", a.modelsPath, err)
	} else {
		fmt.Fprintf(a.out, "models.Load(%s): OK\n", a.modelsPath)
	}
	if strings.TrimSpace(a.providersDir) != "" {
		if _, err := dslconfig.ValidateProvidersDir(a.providersDir); err != nil {
			fmt.Fprintf(a.out, "providers dir 校验失败 (%s): %v\n", a.providersDir, err)
		} else {
			fmt.Fprintf(a.out, "providers dir 校验: OK (%s)\n", a.providersDir)
		}
	}
	return nil
}

func (a *app) saveAll() error {
	if !a.keysDirty && !a.modelsDirty {
		fmt.Fprintln(a.out, "没有需要保存的更改。")
		return nil
	}
	if a.keysDirty {
		if err := validateKeysDoc(a.keysDoc); err != nil {
			return err
		}
		b, err := encodeYAML(a.keysDoc)
		if err != nil {
			return err
		}
		if err := writeAtomic(a.keysPath, b, a.backup); err != nil {
			return err
		}
		a.keysDirty = false
		fmt.Fprintln(a.out, "已保存 keys.yaml。")
	}
	if a.modelsDirty {
		if err := validateModelsDoc(a.modelsDoc); err != nil {
			return err
		}
		b, err := encodeYAML(a.modelsDoc)
		if err != nil {
			return err
		}
		if err := writeAtomic(a.modelsPath, b, a.backup); err != nil {
			return err
		}
		a.modelsDirty = false
		fmt.Fprintln(a.out, "已保存 models.yaml。")
	}
	return nil
}

func (a *app) readChoice(min, max int) (int, error) {
	for {
		s, err := a.readLine("选择: ")
		if err != nil {
			return 0, err
		}
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			continue
		}
		if n < min || n > max {
			continue
		}
		return n, nil
	}
}

func (a *app) readMenuChoice(prompt string, allows []string) (string, error) {
	set := make(map[string]struct{}, len(allows))
	for _, v := range allows {
		set[strings.ToLower(strings.TrimSpace(v))] = struct{}{}
	}
	for {
		s, err := a.readLine(prompt)
		if err != nil {
			return "", err
		}
		s = strings.ToLower(strings.TrimSpace(s))
		if _, ok := set[s]; ok {
			return s, nil
		}
	}
}

func (a *app) readInt(prompt string) (int, error) {
	for {
		s, err := a.readLine(prompt)
		if err != nil {
			return 0, err
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return 0, nil
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			continue
		}
		return n, nil
	}
}

func (a *app) confirm(prompt string) (bool, error) {
	s, err := a.readLine(prompt)
	if err != nil {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

func (a *app) readLine(prompt string) (string, error) {
	fmt.Fprint(a.out, prompt)
	s, err := a.in.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(s, "\r\n"), nil
}

func loadConfigIfExists(path string) (*config.Config, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return nil, nil
	}
	if _, err := os.Stat(p); err != nil {
		return nil, err
	}
	return config.Load(p)
}

func loadOrInitKeysDoc(path string) (*yaml.Node, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return nil, errors.New("missing keys path")
	}
	b, err := os.ReadFile(p) // #nosec G304 -- admin tool reads user-provided file.
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return initEmptyKeysDoc(), nil
		}
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	if doc.Kind == 0 {
		return initEmptyKeysDoc(), nil
	}
	ensureProvidersMap(&doc)
	return &doc, nil
}

func loadOrInitModelsDoc(path string) (*yaml.Node, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return nil, errors.New("missing models path")
	}
	b, err := os.ReadFile(p) // #nosec G304 -- admin tool reads user-provided file.
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return initEmptyModelsDoc(), nil
		}
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	if doc.Kind == 0 {
		return initEmptyModelsDoc(), nil
	}
	ensureModelsMap(&doc)
	return &doc, nil
}

func initEmptyKeysDoc() *yaml.Node {
	doc := &yaml.Node{Kind: yaml.DocumentNode}
	m := &yaml.Node{Kind: yaml.MappingNode}
	doc.Content = []*yaml.Node{m}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "providers", Tag: "!!str"},
		&yaml.Node{Kind: yaml.MappingNode},
	)
	return doc
}

func initEmptyModelsDoc() *yaml.Node {
	doc := &yaml.Node{Kind: yaml.DocumentNode}
	m := &yaml.Node{Kind: yaml.MappingNode}
	doc.Content = []*yaml.Node{m}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "models", Tag: "!!str"},
		&yaml.Node{Kind: yaml.MappingNode},
	)
	return doc
}

func ensureProvidersMap(doc *yaml.Node) {
	if doc == nil {
		return
	}
	if doc.Kind != yaml.DocumentNode {
		return
	}
	if len(doc.Content) == 0 || doc.Content[0] == nil {
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return
	}
	if _, ok := mappingGet(root, "providers"); ok {
		return
	}
	mappingSet(root, "providers", &yaml.Node{Kind: yaml.MappingNode})
}

func ensureModelsMap(doc *yaml.Node) {
	if doc == nil {
		return
	}
	if doc.Kind != yaml.DocumentNode {
		return
	}
	if len(doc.Content) == 0 || doc.Content[0] == nil {
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return
	}
	if _, ok := mappingGet(root, "models"); ok {
		return
	}
	mappingSet(root, "models", &yaml.Node{Kind: yaml.MappingNode})
}

type keyUpdate struct {
	Name            *string
	Value           *string
	BaseURLOverride *string
}

type accessKeyUpdate struct {
	Name     *string
	Value    *string
	Disabled *bool
	Comment  *string
}

type modelUpdate struct {
	Providers *[]string
	Strategy  *string
	OwnedBy   *string
}

func ptr[T any](v T) *T { return &v }

func parseProviders(s string) []string {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\t' || r == ' '
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func encodeYAML(doc *yaml.Node) ([]byte, error) {
	var sb strings.Builder
	enc := yaml.NewEncoder(&sb)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		_ = enc.Close()
		return nil, err
	}
	_ = enc.Close()
	return []byte(sb.String()), nil
}

func writeAtomic(path string, data []byte, backup bool) error {
	p := strings.TrimSpace(path)
	if p == "" {
		return errors.New("missing path")
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	if backup {
		if _, err := os.Stat(p); err == nil {
			ts := time.Now().Format("20060102-150405")
			bpath := p + ".bak." + ts
			if err := copyFile(p, bpath); err != nil {
				return err
			}
		}
	}

	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src) // #nosec G304 -- admin tool reads user-provided file.
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func valueHint(v string) string {
	s := strings.TrimSpace(v)
	if s == "" {
		return "(empty)"
	}
	if strings.HasPrefix(s, "ENC[") {
		return "(ENC[...])"
	}
	if len(s) <= 8 {
		return fmt.Sprintf("%q", s)
	}
	return fmt.Sprintf("%q...(%d)", s[:4], len(s))
}

// YAML helpers (order-preserving)

func mappingGet(m *yaml.Node, key string) (*yaml.Node, bool) {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil, false
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k := m.Content[i]
		v := m.Content[i+1]
		if k != nil && k.Value == key {
			return v, true
		}
	}
	return nil, false
}

func mappingSet(m *yaml.Node, key string, val *yaml.Node) {
	if m == nil || m.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k := m.Content[i]
		if k != nil && k.Value == key {
			m.Content[i+1] = val
			return
		}
	}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"},
		val,
	)
}

func mappingDel(m *yaml.Node, key string) bool {
	if m == nil || m.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k := m.Content[i]
		if k != nil && k.Value == key {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return true
		}
	}
	return false
}

func providersMap(doc *yaml.Node) (*yaml.Node, error) {
	if doc == nil || doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0] == nil {
		return nil, errors.New("invalid yaml doc")
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, errors.New("root is not mapping")
	}
	pn, ok := mappingGet(root, "providers")
	if !ok || pn == nil {
		pn = &yaml.Node{Kind: yaml.MappingNode}
		mappingSet(root, "providers", pn)
	}
	if pn.Kind != yaml.MappingNode {
		return nil, errors.New("providers is not mapping")
	}
	return pn, nil
}

func modelsMap(doc *yaml.Node) (*yaml.Node, error) {
	if doc == nil || doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0] == nil {
		return nil, errors.New("invalid yaml doc")
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, errors.New("root is not mapping")
	}
	mn, ok := mappingGet(root, "models")
	if !ok || mn == nil {
		mn = &yaml.Node{Kind: yaml.MappingNode}
		mappingSet(root, "models", mn)
	}
	if mn.Kind != yaml.MappingNode {
		return nil, errors.New("models is not mapping")
	}
	return mn, nil
}

func listProviders(doc *yaml.Node) []string {
	pm, err := providersMap(doc)
	if err != nil {
		return nil
	}
	var out []string
	for i := 0; i+1 < len(pm.Content); i += 2 {
		k := pm.Content[i]
		if k != nil && strings.TrimSpace(k.Value) != "" {
			out = append(out, strings.TrimSpace(k.Value))
		}
	}
	return out
}

func listModelIDs(doc *yaml.Node) []string {
	mm, err := modelsMap(doc)
	if err != nil {
		return nil
	}
	var out []string
	for i := 0; i+1 < len(mm.Content); i += 2 {
		k := mm.Content[i]
		if k != nil && strings.TrimSpace(k.Value) != "" {
			out = append(out, strings.TrimSpace(k.Value))
		}
	}
	return out
}

func getProviderNode(doc *yaml.Node, provider string) (*yaml.Node, bool) {
	pm, err := providersMap(doc)
	if err != nil {
		return nil, false
	}
	want := strings.TrimSpace(provider)
	for i := 0; i+1 < len(pm.Content); i += 2 {
		k := pm.Content[i]
		v := pm.Content[i+1]
		if k != nil && strings.TrimSpace(k.Value) == want {
			return v, true
		}
	}
	return nil, false
}

func getModelNode(doc *yaml.Node, modelID string) (*yaml.Node, bool) {
	mm, err := modelsMap(doc)
	if err != nil {
		return nil, false
	}
	want := strings.TrimSpace(modelID)
	for i := 0; i+1 < len(mm.Content); i += 2 {
		k := mm.Content[i]
		v := mm.Content[i+1]
		if k != nil && strings.TrimSpace(k.Value) == want {
			return v, true
		}
	}
	return nil, false
}

func ensureProviderNode(doc *yaml.Node, provider string) *yaml.Node {
	pm, _ := providersMap(doc)
	p := strings.TrimSpace(provider)
	if p == "" {
		return nil
	}
	if n, ok := getProviderNode(doc, p); ok && n != nil {
		return n
	}
	// Append at end to preserve existing order.
	provNode := &yaml.Node{Kind: yaml.MappingNode}
	keysSeq := &yaml.Node{Kind: yaml.SequenceNode}
	mappingSet(provNode, "keys", keysSeq)
	pm.Content = append(pm.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: p, Tag: "!!str"},
		provNode,
	)
	return provNode
}

func ensureModelNode(doc *yaml.Node, modelID string) *yaml.Node {
	mm, _ := modelsMap(doc)
	id := strings.TrimSpace(modelID)
	if id == "" {
		return nil
	}
	if n, ok := getModelNode(doc, id); ok && n != nil {
		return n
	}
	rtNode := &yaml.Node{Kind: yaml.MappingNode}
	mm.Content = append(mm.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: id, Tag: "!!str"},
		rtNode,
	)
	return rtNode
}

func deleteProvider(doc *yaml.Node, provider string) error {
	pm, err := providersMap(doc)
	if err != nil {
		return err
	}
	want := strings.TrimSpace(provider)
	for i := 0; i+1 < len(pm.Content); i += 2 {
		k := pm.Content[i]
		if k != nil && strings.TrimSpace(k.Value) == want {
			pm.Content = append(pm.Content[:i], pm.Content[i+2:]...)
			return nil
		}
	}
	return fmt.Errorf("provider not found: %s", want)
}

func deleteModel(doc *yaml.Node, modelID string) error {
	mm, err := modelsMap(doc)
	if err != nil {
		return err
	}
	want := strings.TrimSpace(modelID)
	for i := 0; i+1 < len(mm.Content); i += 2 {
		k := mm.Content[i]
		if k != nil && strings.TrimSpace(k.Value) == want {
			mm.Content = append(mm.Content[:i], mm.Content[i+2:]...)
			return nil
		}
	}
	return fmt.Errorf("model not found: %s", want)
}

func getModelRoute(doc *yaml.Node, modelID string) (models.Route, bool) {
	n, ok := getModelNode(doc, modelID)
	if !ok || n == nil || n.Kind != yaml.MappingNode {
		return models.Route{}, false
	}
	rt := models.Route{}
	if v, ok := mappingGet(n, "providers"); ok && v != nil && v.Kind == yaml.SequenceNode {
		for _, it := range v.Content {
			if it == nil {
				continue
			}
			p := strings.ToLower(strings.TrimSpace(it.Value))
			if p != "" {
				rt.Providers = append(rt.Providers, p)
			}
		}
	}
	if v, ok := mappingGet(n, "strategy"); ok && v != nil {
		rt.Strategy = models.Strategy(strings.TrimSpace(v.Value))
	}
	if v, ok := mappingGet(n, "owned_by"); ok && v != nil {
		rt.OwnedBy = strings.TrimSpace(v.Value)
	}
	return rt, true
}

func setModelRoute(doc *yaml.Node, modelID string, rt models.Route) error {
	n := ensureModelNode(doc, modelID)
	if n == nil {
		return errors.New("invalid model id")
	}
	if n.Kind != yaml.MappingNode {
		return errors.New("model route is not mapping")
	}
	// providers
	seq := &yaml.Node{Kind: yaml.SequenceNode}
	for _, p := range rt.Providers {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: p})
	}
	mappingSet(n, "providers", seq)
	// strategy
	if strings.TrimSpace(string(rt.Strategy)) != "" {
		mappingSet(n, "strategy", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(string(rt.Strategy))})
	}
	// owned_by
	if strings.TrimSpace(rt.OwnedBy) != "" {
		mappingSet(n, "owned_by", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(rt.OwnedBy)})
	}
	return nil
}

func updateModelRoute(doc *yaml.Node, modelID string, up modelUpdate) error {
	n, ok := getModelNode(doc, modelID)
	if !ok || n == nil {
		return fmt.Errorf("model not found: %s", strings.TrimSpace(modelID))
	}
	if n.Kind != yaml.MappingNode {
		return errors.New("model route is not mapping")
	}
	if up.Providers != nil {
		if *up.Providers == nil {
			mappingDel(n, "providers")
		} else {
			seq := &yaml.Node{Kind: yaml.SequenceNode}
			for _, p := range *up.Providers {
				p = strings.ToLower(strings.TrimSpace(p))
				if p == "" {
					continue
				}
				seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: p})
			}
			mappingSet(n, "providers", seq)
		}
	}
	if up.Strategy != nil {
		if strings.TrimSpace(*up.Strategy) == "" {
			mappingDel(n, "strategy")
		} else {
			mappingSet(n, "strategy", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(*up.Strategy)})
		}
	}
	if up.OwnedBy != nil {
		if strings.TrimSpace(*up.OwnedBy) == "" {
			mappingDel(n, "owned_by")
		} else {
			mappingSet(n, "owned_by", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(*up.OwnedBy)})
		}
	}
	return nil
}

func providerKeysSeq(doc *yaml.Node, provider string) (*yaml.Node, error) {
	pn := ensureProviderNode(doc, provider)
	if pn == nil || pn.Kind != yaml.MappingNode {
		return nil, errors.New("invalid provider node")
	}
	kn, ok := mappingGet(pn, "keys")
	if !ok || kn == nil {
		kn = &yaml.Node{Kind: yaml.SequenceNode}
		mappingSet(pn, "keys", kn)
	}
	if kn.Kind != yaml.SequenceNode {
		return nil, errors.New("provider.keys is not sequence")
	}
	return kn, nil
}

func listProviderKeys(doc *yaml.Node, provider string) ([]keystore.Key, error) {
	seq, err := providerKeysSeq(doc, provider)
	if err != nil {
		return nil, err
	}
	var out []keystore.Key
	for _, it := range seq.Content {
		if it == nil || it.Kind != yaml.MappingNode {
			continue
		}
		k := keystore.Key{}
		if v, ok := mappingGet(it, "name"); ok && v != nil {
			k.Name = strings.TrimSpace(v.Value)
		}
		if v, ok := mappingGet(it, "value"); ok && v != nil {
			k.Value = strings.TrimSpace(v.Value)
		}
		if v, ok := mappingGet(it, "base_url_override"); ok && v != nil {
			k.BaseURLOverride = strings.TrimSpace(v.Value)
		}
		out = append(out, k)
	}
	return out, nil
}

func appendProviderKey(doc *yaml.Node, provider string, k keystore.Key) error {
	seq, err := providerKeysSeq(doc, provider)
	if err != nil {
		return err
	}
	m := &yaml.Node{Kind: yaml.MappingNode}
	if strings.TrimSpace(k.Name) != "" {
		mappingSet(m, "name", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(k.Name)})
	}
	mappingSet(m, "value", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(k.Value)})
	if strings.TrimSpace(k.BaseURLOverride) != "" {
		mappingSet(m, "base_url_override", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(k.BaseURLOverride)})
	}
	seq.Content = append(seq.Content, m)
	return nil
}

func updateProviderKey(doc *yaml.Node, provider string, index int, up keyUpdate) error {
	seq, err := providerKeysSeq(doc, provider)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(seq.Content) {
		return errors.New("index out of range")
	}
	it := seq.Content[index]
	if it == nil || it.Kind != yaml.MappingNode {
		return errors.New("invalid key node")
	}
	if up.Name != nil {
		if strings.TrimSpace(*up.Name) == "" {
			mappingDel(it, "name")
		} else {
			mappingSet(it, "name", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(*up.Name)})
		}
	}
	if up.Value != nil {
		mappingSet(it, "value", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(*up.Value)})
	}
	if up.BaseURLOverride != nil {
		if strings.TrimSpace(*up.BaseURLOverride) == "" {
			mappingDel(it, "base_url_override")
		} else {
			mappingSet(it, "base_url_override", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(*up.BaseURLOverride)})
		}
	}
	return nil
}

func deleteProviderKey(doc *yaml.Node, provider string, index int) error {
	seq, err := providerKeysSeq(doc, provider)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(seq.Content) {
		return errors.New("index out of range")
	}
	seq.Content = append(seq.Content[:index], seq.Content[index+1:]...)
	return nil
}

func accessKeysSeq(doc *yaml.Node) (*yaml.Node, error) {
	if doc == nil || doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0] == nil {
		return nil, errors.New("invalid yaml doc")
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, errors.New("root is not mapping")
	}
	kn, ok := mappingGet(root, "access_keys")
	if !ok || kn == nil {
		kn = &yaml.Node{Kind: yaml.SequenceNode}
		mappingSet(root, "access_keys", kn)
	}
	if kn.Kind != yaml.SequenceNode {
		return nil, errors.New("access_keys is not sequence")
	}
	return kn, nil
}

func listAccessKeysDoc(doc *yaml.Node) ([]keystore.AccessKey, error) {
	seq, err := accessKeysSeq(doc)
	if err != nil {
		return nil, err
	}
	var out []keystore.AccessKey
	for _, it := range seq.Content {
		if it == nil || it.Kind != yaml.MappingNode {
			continue
		}
		ak := keystore.AccessKey{}
		if v, ok := mappingGet(it, "name"); ok && v != nil {
			ak.Name = strings.TrimSpace(v.Value)
		}
		if v, ok := mappingGet(it, "value"); ok && v != nil {
			ak.Value = strings.TrimSpace(v.Value)
		}
		if v, ok := mappingGet(it, "disabled"); ok && v != nil {
			switch strings.ToLower(strings.TrimSpace(v.Value)) {
			case "true", "y", "yes", "1":
				ak.Disabled = true
			}
		}
		if v, ok := mappingGet(it, "comment"); ok && v != nil {
			ak.Comment = strings.TrimSpace(v.Value)
		}
		out = append(out, ak)
	}
	return out, nil
}

func appendAccessKeyDoc(doc *yaml.Node, ak keystore.AccessKey) error {
	seq, err := accessKeysSeq(doc)
	if err != nil {
		return err
	}
	m := &yaml.Node{Kind: yaml.MappingNode}
	if strings.TrimSpace(ak.Name) != "" {
		mappingSet(m, "name", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(ak.Name)})
	}
	mappingSet(m, "value", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(ak.Value)})
	if ak.Disabled {
		mappingSet(m, "disabled", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"})
	}
	if strings.TrimSpace(ak.Comment) != "" {
		mappingSet(m, "comment", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(ak.Comment)})
	}
	seq.Content = append(seq.Content, m)
	return nil
}

func updateAccessKeyDoc(doc *yaml.Node, index int, up accessKeyUpdate) error {
	seq, err := accessKeysSeq(doc)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(seq.Content) {
		return errors.New("index out of range")
	}
	it := seq.Content[index]
	if it == nil || it.Kind != yaml.MappingNode {
		return errors.New("invalid access_key node")
	}
	if up.Name != nil {
		if strings.TrimSpace(*up.Name) == "" {
			mappingDel(it, "name")
		} else {
			mappingSet(it, "name", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(*up.Name)})
		}
	}
	if up.Value != nil {
		mappingSet(it, "value", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(*up.Value)})
	}
	if up.Disabled != nil {
		if *up.Disabled {
			mappingSet(it, "disabled", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"})
		} else {
			mappingDel(it, "disabled")
		}
	}
	if up.Comment != nil {
		if strings.TrimSpace(*up.Comment) == "" {
			mappingDel(it, "comment")
		} else {
			mappingSet(it, "comment", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(*up.Comment)})
		}
	}
	return nil
}

func deleteAccessKeyDoc(doc *yaml.Node, index int) error {
	seq, err := accessKeysSeq(doc)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(seq.Content) {
		return errors.New("index out of range")
	}
	seq.Content = append(seq.Content[:index], seq.Content[index+1:]...)
	return nil
}

func validateKeysDoc(doc *yaml.Node) error {
	if _, err := providersMap(doc); err != nil {
		return err
	}
	if doc == nil || doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0] == nil {
		return errors.New("invalid yaml doc")
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return errors.New("root is not mapping")
	}
	if akn, ok := mappingGet(root, "access_keys"); ok && akn != nil && akn.Kind != yaml.SequenceNode {
		return errors.New("access_keys is not sequence")
	}
	return nil
}

func validateModelsDoc(doc *yaml.Node) error {
	if _, err := modelsMap(doc); err != nil {
		return err
	}
	b, err := encodeYAML(doc)
	if err != nil {
		return err
	}
	var f models.File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return err
	}
	return nil
}

func upstreamEnvVar(provider, name string, index int) string {
	// Keep consistent with internal/keystore env var behavior.
	p := strings.ToUpper(strings.TrimSpace(provider))
	n := strings.ToUpper(strings.TrimSpace(name))
	if n == "" {
		return fmt.Sprintf("ONR_UPSTREAM_KEY_%s_%d", sanitizeEnvToken(p), index+1)
	}
	return fmt.Sprintf("ONR_UPSTREAM_KEY_%s_%s", sanitizeEnvToken(p), sanitizeEnvToken(n))
}

func accessEnvVar(name string, index int) string {
	n := strings.ToUpper(strings.TrimSpace(name))
	if n == "" {
		return fmt.Sprintf("ONR_ACCESS_KEY_%d", index+1)
	}
	return fmt.Sprintf("ONR_ACCESS_KEY_%s", sanitizeEnvToken(n))
}

func sanitizeEnvToken(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}
