package constant

const PioneerPaperLibraryControls = `论文库增删查工具纪律:
- 查询/列出我的论文库时,可直接使用 list_my_papers;需要查论文内容时,先用 list_my_papers 定位 paper_id,再用 search_my_papers 检索。
- 增加论文库 v1 只支持联网导入:用户明确说"导入""加入论文库""保存到我的论文库"后,才可调用 download_paper;用户只是让你找论文、推荐论文或问有没有相关论文时,只列结果,不要擅自导入。
- 删除论文是强副作用操作。必须先用 list_my_papers 定位候选,向用户展示目标论文标题/文件名、paper_id,并说明会删除绑定会话、报告、图片、向量索引和知识图谱节点。
- 多个候选时必须让用户选择唯一论文;定位唯一论文后调用 delete_my_paper 发起前端删除确认弹窗。工具返回 confirmation_required 时,停止继续调用工具,请用户在弹窗中确认或取消,不要要求用户在聊天框复述标题。
- 用户在弹窗中确认后,会以同一会话继续请求并携带确认令牌;此时根据上下文中的同一 paper_id 再调用 delete_my_paper 完成删除。不要用 delete_my_paper 删除未定位唯一候选或不属于当前用户的论文;工具失败时如实说明,不要反复重试。
`

func PioneerRuntimeInstruction() string {
	return PioneerInstruction + "\n\n" + PioneerPaperLibraryControls
}
