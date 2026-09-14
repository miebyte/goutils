# goutils Skill

`goutils` 用于指导 Codex、Cursor 等 Agent 按 DDD 分层和 `github.com/miebyte/goutils` 的真实 API 创建、扩展或审查 Go 服务。

目录遵循 Agent Skill 的渐进式披露结构：

```text
skills/goutils/
├── SKILL.md              # 入口：核心约束与按需导航
├── README.md             # 安装说明
├── agents/openai.yaml    # Codex UI 元数据
└── references/           # DDD 与各模块细则，按任务读取
```

不要把模块细则搬回各模块 `README.md`；Skill 的规则统一维护在 `references/<module>.md`。

## Codex 安装

用户级安装：

```bash
mkdir -p "$HOME/.agents/skills"
cp -R "$(pwd)/skills/goutils" "$HOME/.agents/skills/goutils"
```

仓库级安装：

```bash
mkdir -p .agents/skills
cp -R /path/to/goutils/skills/goutils .agents/skills/goutils
```

开发时也可将上述目标目录软链接到本仓库的 `skills/goutils`。

## Cursor 安装

项目级安装：

```bash
mkdir -p .cursor/skills
cp -R /path/to/goutils/skills/goutils .cursor/skills/goutils
```

用户级安装：

```bash
mkdir -p "$HOME/.cursor/skills"
cp -R "$(pwd)/skills/goutils" "$HOME/.cursor/skills/goutils"
```

## 验证

```bash
python3 /path/to/skill-creator/scripts/quick_validate.py skills/goutils
```

还应检查：

- `SKILL.md` 的所有相对链接都存在。
- `references/` 中每个目标模块都有同名文档。
- 示例 import 使用 `github.com/miebyte/goutils/...`。
- 文档没有把旧仓库导入路径或旧 API 名称当作当前可调用能力。

## 官方规范

- Codex Agent Skills: https://developers.openai.com/codex/skills
- Cursor Agent Skills: https://cursor.com/docs/context/skills
