package xiaohongshu

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
)

// PublishImageContent 发布图文内容
type PublishImageContent struct {
	Title      string
	Content    string
	ImagePaths []string
}

type PublishAction struct {
	page *rod.Page
}

const (
	urlOfPublic = `https://creator.xiaohongshu.com/publish/publish?source=official`
)

func NewPublishImageAction(page *rod.Page) (*PublishAction, error) {

	pp := page.Timeout(60 * time.Second)

	pp.MustNavigate(urlOfPublic)

	pp.MustElement(`div.upload-content`).MustWaitVisible()
	slog.Info("wait for upload-content visible success")

	// 等待一段时间确保页面完全加载
	time.Sleep(1 * time.Second)

	createElems := pp.MustElements("div.creator-tab")
	slog.Info("foundcreator-tab elements", "count", len(createElems))
	for _, elem := range createElems {
		text, err := elem.Text()
		if err != nil {
			slog.Error("获取元素文本失败", "error", err)
			continue
		}

		if text == "上传图文" {
			if err := elem.Click(proto.InputMouseButtonLeft, 1); err != nil {
				slog.Error("点击元素失败", "error", err)
				continue
			}
			break
		}
	}

	time.Sleep(1 * time.Second)

	return &PublishAction{
		page: pp,
	}, nil
}

func (p *PublishAction) Publish(ctx context.Context, content PublishImageContent) error {
	if len(content.ImagePaths) == 0 {
		return errors.New("图片不能为空")
	}

	page := p.page.Context(ctx)

	if err := uploadImages(page, content.ImagePaths); err != nil {
		return errors.Wrap(err, "小红书上传图片失败")
	}

	if err := submitPublish(page, content.Title, content.Content); err != nil {
		return errors.Wrap(err, "小红书发布失败")
	}

	return nil
}

func uploadImages(page *rod.Page, imagesPaths []string) error {
	pp := page.Timeout(30 * time.Second)

	// 等待上传输入框出现
	uploadInput := pp.MustElement(".upload-input")

	// 上传多个文件
	uploadInput.MustSetFiles(imagesPaths...)

	// 等待上传完成
	time.Sleep(3 * time.Second)

	return nil
}

func submitPublish(page *rod.Page, title, content string) error {

	titleElem := page.MustElement("div.d-input input")
	titleElem.MustInput(title)

	time.Sleep(1 * time.Second)

	if contentElem, ok := getContentElement(page); ok {
		// 使用新的标签处理函数
		if err := inputContentWithTags(contentElem, content); err != nil {
			return errors.Wrap(err, "输入内容失败")
		}
	} else {
		return errors.New("没有找到内容输入框")
	}

	time.Sleep(1 * time.Second)

	submitButton := page.MustElement("div.submit div.d-button-content")
	submitButton.MustClick()

	time.Sleep(3 * time.Second)

	return nil
}

// inputContentWithTags 处理包含标签的内容输入
func inputContentWithTags(contentElem *rod.Element, content string) error {
	// 解析内容，分离普通文本和标签
	parts := parseContentWithTags(content)

	for _, part := range parts {
		if part.IsTag {
			// 处理标签
			if err := inputTag(contentElem, part.Text); err != nil {
				slog.Warn("处理标签失败，作为普通文本输入", "tag", part.Text, "error", err)
				// 如果标签处理失败，作为普通文本输入
				contentElem.MustInput(part.Text)
			}
		} else {
			// 输入普通文本
			contentElem.MustInput(part.Text)
		}
		// 短暂延迟，确保输入被正确处理
		time.Sleep(200 * time.Millisecond)
	}

	return nil
}

// ContentPart 内容片段
type ContentPart struct {
	Text  string
	IsTag bool
}

// parseContentWithTags 解析内容，分离普通文本和标签
func parseContentWithTags(content string) []ContentPart {
	var parts []ContentPart

	// 使用正则表达式匹配标签 #标签名
	tagRegex := regexp.MustCompile(`#([^\s#]+)`)

	lastIndex := 0
	matches := tagRegex.FindAllStringSubmatchIndex(content, -1)

	for _, match := range matches {
		// 添加标签前的普通文本
		if match[0] > lastIndex {
			normalText := content[lastIndex:match[0]]
			if normalText != "" {
				parts = append(parts, ContentPart{Text: normalText, IsTag: false})
			}
		}

		// 添加标签
		tagText := content[match[0]:match[1]] // 包含#的完整标签
		parts = append(parts, ContentPart{Text: tagText, IsTag: true})

		lastIndex = match[1]
	}

	// 添加最后剩余的普通文本
	if lastIndex < len(content) {
		remainingText := content[lastIndex:]
		if remainingText != "" {
			parts = append(parts, ContentPart{Text: remainingText, IsTag: false})
		}
	}

	// 如果没有找到标签，整个内容作为普通文本
	if len(parts) == 0 {
		parts = append(parts, ContentPart{Text: content, IsTag: false})
	}

	return parts
}

// inputTag 输入标签并处理下拉框
func inputTag(contentElem *rod.Element, tagText string) error {
	// 输入 # 符号触发标签模式
	contentElem.MustInput("#")
	time.Sleep(500 * time.Millisecond) // 等待下拉框出现

	// 输入标签名称（去掉#符号）
	tagName := strings.TrimPrefix(tagText, "#")
	contentElem.MustInput(tagName)
	time.Sleep(800 * time.Millisecond) // 等待下拉框更新

	// 尝试选择第一个标签选项
	if err := selectFirstTagOption(contentElem); err != nil {
		slog.Warn("选择标签选项失败", "error", err)
	}

	return nil
}

// selectFirstTagOption 选择第一个标签选项
func selectFirstTagOption(contentElem *rod.Element) error {
	// 等待下拉框出现
	time.Sleep(300 * time.Millisecond)

	// 如果找不到下拉框选项，尝试按回车键确认
	contentElem.MustType(input.Enter)
	time.Sleep(200 * time.Millisecond)

	return errors.New("未找到标签下拉框选项")
}

// 查找内容输入框 - 使用Race方法处理两种样式
func getContentElement(page *rod.Page) (*rod.Element, bool) {
	var foundElement *rod.Element
	var found bool

	page.Race().
		Element("div.ql-editor").MustHandle(func(e *rod.Element) {
		foundElement = e
		found = true
	}).
		ElementFunc(func(page *rod.Page) (*rod.Element, error) {
			return findTextboxByPlaceholder(page)
		}).MustHandle(func(e *rod.Element) {
		foundElement = e
		found = true
	}).
		MustDo()

	if found {
		return foundElement, true
	}

	slog.Warn("no content element found by any method")
	return nil, false
}

func findTextboxByPlaceholder(page *rod.Page) (*rod.Element, error) {
	elements := page.MustElements("p")
	if elements == nil {
		return nil, errors.New("no p elements found")
	}

	// 查找包含指定placeholder的元素
	placeholderElem := findPlaceholderElement(elements, "输入正文描述")
	if placeholderElem == nil {
		return nil, errors.New("no placeholder element found")
	}

	// 向上查找textbox父元素
	textboxElem := findTextboxParent(placeholderElem)
	if textboxElem == nil {
		return nil, errors.New("no textbox parent found")
	}

	return textboxElem, nil
}

func findPlaceholderElement(elements []*rod.Element, searchText string) *rod.Element {
	for _, elem := range elements {
		placeholder, err := elem.Attribute("data-placeholder")
		if err != nil || placeholder == nil {
			continue
		}

		if strings.Contains(*placeholder, searchText) {
			return elem
		}
	}
	return nil
}

func findTextboxParent(elem *rod.Element) *rod.Element {
	currentElem := elem
	for i := 0; i < 5; i++ {
		parent, err := currentElem.Parent()
		if err != nil {
			break
		}

		role, err := parent.Attribute("role")
		if err != nil || role == nil {
			currentElem = parent
			continue
		}

		if *role == "textbox" {
			return parent
		}

		currentElem = parent
	}
	return nil
}
