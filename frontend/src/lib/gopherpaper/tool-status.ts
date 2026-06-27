type ToolStatusRule = {
  emoji: string;
  label?: string;
  names: string[];
};

const TOOL_STATUS_RULES: ToolStatusRule[] = [
  { emoji: "🔎", label: "论文检索", names: ["search_paper", "论文知识库"] },
  { emoji: "🔎", label: "检索我的论文", names: ["search_my_papers", "检索我的论文"] },
  { emoji: "🖼️", label: "图表检索", names: ["find_figures", "图表与表格"] },
  { emoji: "📚", label: "我的论文列表", names: ["list_my_papers", "我的论文列表"] },
  { emoji: "🗑️", label: "删除我的论文", names: ["delete_my_paper", "删除我的论文"] },
  { emoji: "📥", label: "下载论文", names: ["download_paper", "下载论文"] },
  { emoji: "🌐", label: "联网搜索", names: ["web_search", "联网搜索"] },
  { emoji: "📍", label: "地点定位", names: ["geocode", "地点定位"] },
  { emoji: "🕒", label: "当前时间", names: ["current_time", "当前时间"] },
  { emoji: "🧪", label: "arXiv 检索", names: ["search_arxiv", "arxiv 检索"] },
  { emoji: "🏛️", label: "官方会议录检索", names: ["search_conference_proceedings", "官方会议录检索"] },
  { emoji: "📝", label: "OpenReview 检索", names: ["search_openreview_papers", "openreview 检索"] },
  { emoji: "🎓", label: "学术检索", names: ["search_semantic_scholar", "学术检索"] },
  { emoji: "🧭", label: "相似论文推荐", names: ["recommend_similar_papers", "相似论文推荐"] },
  { emoji: "🔗", label: "查被引论文", names: ["get_paper_citations", "查被引论文"] },
  { emoji: "🧾", label: "查参考文献", names: ["get_paper_references", "查参考文献"] },
  { emoji: "📍", label: "查询瑞幸门店", names: ["queryshoplist", "查询瑞幸门店"] },
  { emoji: "☕", label: "搜索瑞幸商品", names: ["searchproductformcp", "搜索瑞幸商品"] },
  { emoji: "☕", label: "查看商品定制项", names: ["queryproductdetailinfo", "查看商品定制项"] },
  { emoji: "☕", label: "切换商品规格", names: ["switchproduct", "切换商品规格"] },
  { emoji: "🛒", label: "预览订单", names: ["previeworder", "预览订单"] },
  { emoji: "🛒", label: "提交订单", names: ["createorder", "提交订单"] },
  { emoji: "📦", label: "查询订单状态", names: ["queryorderdetailinfo", "查询订单状态"] },
  { emoji: "🛑", label: "取消订单", names: ["cancelorder", "取消订单"] },
  { emoji: "☕", label: "瑞幸工具", names: ["my-coffee", "瑞幸"] },
  { emoji: "🧩", names: ["skill", "Skill"] },
];

function toolStatus(tool: string): { emoji: string; name: string } {
  const name = tool.trim() || "工具";
  const normalized = tool.trim().toLowerCase();
  if (!normalized) return { emoji: "🛠️", name };
  const rule = TOOL_STATUS_RULES.find(({ names }) =>
    names.some((name) => normalized.includes(name.toLowerCase())),
  );
  return { emoji: rule?.emoji ?? "🛠️", name: rule?.label ?? name };
}

export function toolStatusText(tool: string, done: boolean): string {
  const { emoji, name } = toolStatus(tool);
  return done ? `${emoji} ${name} 已返回，正在继续…` : `${emoji} 正在调用 ${name} …`;
}
