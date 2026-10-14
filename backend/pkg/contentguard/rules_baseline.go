package contentguard

// rule 单条词表规则。Pattern 为小写子串（拉丁字符统一按小写匹配）。
type rule struct {
	ID       string
	Category string
	Pattern  string
}

// baselineRules 基线词表（示例级，非权威词表）。
//
// 来源与生效时间：本项目自建基线，只收录明确无争议的违禁/违规词；
// 生产环境必须替换为可追踪来源的权威词表，或按 Plan B 接入第三方审核 API。
// 新增/修改规则时必须在此登记来源与生效时间（AGENTS.md #14、#16）。
var baselineRules = []rule{
	// 违法违规：违禁品与违法服务（考试类产品需重点关注代考/办证/作弊服务）
	{ID: "illegal.drugs", Category: CategoryIllegal, Pattern: "毒品"},
	{ID: "illegal.drugs_ice", Category: CategoryIllegal, Pattern: "冰毒"},
	{ID: "illegal.drugs_heroin", Category: CategoryIllegal, Pattern: "海洛因"},
	{ID: "illegal.gun", Category: CategoryIllegal, Pattern: "枪支交易"},
	{ID: "illegal.gambling_website", Category: CategoryIllegal, Pattern: "赌博网站"},
	{ID: "illegal.gambling_platform", Category: CategoryIllegal, Pattern: "博彩平台"},
	{ID: "illegal.exam_fraud_ghost", Category: CategoryIllegal, Pattern: "代考"},
	{ID: "illegal.exam_fraud_substitute", Category: CategoryIllegal, Pattern: "替考"},
	{ID: "illegal.cert_forgery", Category: CategoryIllegal, Pattern: "办证"},
	{ID: "illegal.money_laundering", Category: CategoryIllegal, Pattern: "洗钱"},
	{ID: "illegal.fraud_brushing", Category: CategoryIllegal, Pattern: "刷单返利"},

	// 色情低俗
	{ID: "porn.general", Category: CategoryPorn, Pattern: "色情"},
	{ID: "porn.adult_video", Category: CategoryPorn, Pattern: "成人视频"},
	{ID: "porn.chat", Category: CategoryPorn, Pattern: "裸聊"},

	// 暴力恐怖
	{ID: "violence.bomb_make", Category: CategoryViolence, Pattern: "制造炸弹"},
	{ID: "violence.explosive_buy", Category: CategoryViolence, Pattern: "购买炸药"},
	{ID: "violence.terror_attack", Category: CategoryViolence, Pattern: "恐怖袭击"},

	// 极端辱骂与威胁
	{ID: "abuse.insult_sb", Category: CategoryAbuse, Pattern: "傻逼"},
	{ID: "abuse.insult_naocan", Category: CategoryAbuse, Pattern: "脑残"},
	{ID: "abuse.threat_die", Category: CategoryAbuse, Pattern: "去死吧"},
}
