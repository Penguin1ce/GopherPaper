export type ToolTone =
  | "paper"
  | "figure"
  | "library"
  | "web"
  | "academic"
  | "location"
  | "time"
  | "coffee"
  | "order"
  | "destructive"
  | "skill"
  | "utility"
  | "default";

type ToolStatusRule = {
  emoji: string;
  label?: string;
  names: string[];
  tone: ToolTone;
};

export interface ToolDisplayInfo {
  raw: string;
  name: string;
  tone: ToolTone;
  matched: boolean;
}

const TOOL_STATUS_RULES: ToolStatusRule[] = [
  { emoji: "🔎", label: "论文检索", names: ["search_paper", "检索论文正文", "论文知识库"], tone: "paper" },
  { emoji: "🔎", label: "检索我的论文", names: ["search_my_papers", "检索我的论文"], tone: "paper" },
  { emoji: "🖼️", label: "图表检索", names: ["find_figures", "检索论文图表", "图表与表格"], tone: "figure" },
  { emoji: "📚", label: "我的论文列表", names: ["list_my_papers", "我的论文列表"], tone: "library" },
  { emoji: "🗑️", label: "删除我的论文", names: ["delete_my_paper", "删除我的论文"], tone: "destructive" },
  { emoji: "📥", label: "下载论文", names: ["download_paper", "下载论文"], tone: "utility" },
  { emoji: "🧭", label: "生成思路图", names: ["generate_paper_flow", "生成思路图"], tone: "paper" },
  { emoji: "🌐", label: "联网搜索", names: ["web_search", "联网搜索"], tone: "web" },
  { emoji: "📍", label: "地点定位", names: ["geocode", "地点定位"], tone: "location" },
  { emoji: "🕒", label: "当前时间", names: ["current_time", "当前时间"], tone: "time" },
  { emoji: "🧪", label: "arXiv 检索", names: ["search_arxiv", "arxiv 检索"], tone: "academic" },
  { emoji: "🏛️", label: "官方会议录检索", names: ["search_conference_proceedings", "官方会议录检索"], tone: "academic" },
  { emoji: "📝", label: "OpenReview 检索", names: ["search_openreview_papers", "openreview 检索"], tone: "academic" },
  { emoji: "🎓", label: "OpenAlex 检索", names: ["search_openalex", "openalex 检索"], tone: "academic" },
  { emoji: "🎓", label: "SciVerse 检索", names: ["search_sciverse", "sciverse 语义检索"], tone: "academic" },
  { emoji: "📖", label: "SciVerse 续读", names: ["read_sciverse_content", "sciverse 原文续读"], tone: "academic" },
  { emoji: "🎓", label: "学术检索", names: ["search_semantic_scholar", "学术检索"], tone: "academic" },
  { emoji: "🧭", label: "相似论文推荐", names: ["recommend_similar_papers", "相似论文推荐"], tone: "academic" },
  { emoji: "🔗", label: "查被引论文", names: ["get_paper_citations", "查被引论文"], tone: "academic" },
  { emoji: "🧾", label: "查参考文献", names: ["get_paper_references", "查参考文献"], tone: "academic" },
  { emoji: "📍", label: "查询瑞幸门店", names: ["queryshoplist", "查询瑞幸门店"], tone: "location" },
  { emoji: "☕", label: "搜索瑞幸商品", names: ["searchproductformcp", "搜索瑞幸商品"], tone: "coffee" },
  { emoji: "☕", label: "查看商品定制项", names: ["queryproductdetailinfo", "查看商品定制项"], tone: "coffee" },
  { emoji: "☕", label: "切换商品规格", names: ["switchproduct", "切换商品规格"], tone: "coffee" },
  { emoji: "🛒", label: "预览订单", names: ["previeworder", "预览订单"], tone: "order" },
  { emoji: "🛒", label: "提交订单", names: ["createorder", "提交订单"], tone: "order" },
  { emoji: "📦", label: "查询订单状态", names: ["queryorderdetailinfo", "查询订单状态"], tone: "order" },
  { emoji: "🛑", label: "取消订单", names: ["cancelorder", "取消订单"], tone: "destructive" },
  { emoji: "☕", label: "瑞幸工具", names: ["my-coffee", "瑞幸"], tone: "coffee" },
  { emoji: "🧩", names: ["skill", "Skill"], tone: "skill" },
];

function findToolRule(tool: string) {
  const normalized = tool.trim().toLowerCase();
  if (!normalized) return undefined;
  return TOOL_STATUS_RULES.find(({ names }) =>
    names.some((name) => normalized.includes(name.toLowerCase())),
  );
}

export function toolDisplayInfo(tool: string): ToolDisplayInfo {
  const name = tool.trim() || "工具";
  const rule = findToolRule(name);
  return {
    raw: name,
    name: rule?.label ?? name,
    tone: rule?.tone ?? "default",
    matched: Boolean(rule),
  };
}

function toolStatus(tool: string): { emoji: string; name: string } {
  const info = toolDisplayInfo(tool);
  const rule = findToolRule(info.raw);
  return { emoji: rule?.emoji ?? "🛠️", name: info.name };
}

export function toolStatusText(tool: string, done: boolean): string {
  const { emoji, name } = toolStatus(tool);
  return done ? `${emoji} ${name} 已返回，正在继续…` : `${emoji} 正在调用 ${name} …`;
}
