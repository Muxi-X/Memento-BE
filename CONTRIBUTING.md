### 通用约定

1. `main` 为线上稳定分支
2. `develop` 为日常集成分支
3. 合并策略统一使用 squash merge（保持 `develop`/`main` 历史整洁）

### 提交前自检

1. 本项目提供统一命令入口：`Makefile`
2. 提交前请在本地至少通过：`make fmt`、`make lint`、`make test`
3. 如需与 CI 对齐，以仓库 CI 配置中的命令为准

### Commit 规范

1. 仓库合并策略为 squash merge：最终落到 `main`/`develop` 的 commit 标题通常等于 PR 标题，所以 PR 标题必须符合本规范
2. 采用 Conventional Commits：`<type>(<scope>): <subject>`（`scope` 可选）
3. `type` 建议范围：`feat`、`fix`、`refactor`、`perf`、`test`、`docs`、`chore`、`build`、`ci`、`revert`
4. 如存在破坏性变更：在 commit/PR 描述中标注 `BREAKING CHANGE:`，并提供迁移说明
5. 示例
6. `feat(auth): add email login flow`
7. `fix(db): prevent duplicate migrations`
8. `refactor(transport): simplify v1 error mapping`

### Pull Request 要求

1. PR 描述需包含：变更背景、主要改动、影响面、验证方式（含本地执行的 `make ...` 结果）
2. 功能开发与 Bug 修复必须补齐对应测试或说明不可测原因与手工验证步骤
3. PR 标题需符合 Conventional Commits（见上方 Commit 规范）
4. CI Checks 必须全绿方可合并

### CI 失败如何处理

1. CI 失败默认视为可复现问题：在本地修复后 push 新提交触发 CI
2. 禁止通过反复 rerun “碰运气过关”
3. 允许重新触发一次 CI：仅限已确认失败原因是 CI 平台抖动或外部依赖短暂不可用（需要在 PR 里注明原因与链接/截图）
4. 如果仍失败：按真实失败原因修复或等待外部依赖恢复后再由新提交触发

### 功能开发

1. 从 `develop` 分支 checkout 一个新的功能分支，如 `feature/new-feat`
2. 完成功能开发，并写好测试
3. 推送分支，向 `develop` 分支发起 Pull Request
4. 等待 CI 流水线全部通过，使用 squash merge 完成合并

### 版本发布

1. 从 `develop` 分支 checkout 一个新的发布分支，如 `release/v0.1.0`
2. 完成 bug 修复、文档更新，并准备本次版本发布说明
3. 推送分支，向 `main` 分支发起 Pull Request
4. 等待 CD 流水线全部通过，使用 squash merge 完成合并
5. 将发布分支的变更回灌到 `develop`（向 `develop` 再发起一个 PR，使用 squash merge 完成合并）

### 版本发布细化

1. 版本号采用 SemVer：`vMAJOR.MINOR.PATCH`
2. 版本唯一来源为 Git tag：在 `main` 合并提交上创建 tag（例如 `v0.1.0`）
3. 发布分支（如 `release/v0.1.0`）上只允许：bug 修复、文档更新、必要的配置调整（避免继续加功能）
4. 发布前建议本地至少跑一遍：`make -f Makefile/makefile fmt`、`make -f Makefile/makefile lint`、`make -f Makefile/makefile test`
5. 涉及关键链路或数据库变更时建议额外跑：`make -f Makefile/makefile it`
6. `main` 合并完成并打 tag 后，必须将同一批变更回灌到 `develop`

### Bug修复

1. 从 `develop` 分支 checkout 一个新的 bug 修复分支，如 `bugfix/auth`
2. 完成 bug 修复
3. 推送分支，向 `develop` 分支发起 Pull Request
4. 等待 CI 流水线全部通过，使用 squash merge 完成合并

### 热修复

1. 从 `main` 分支 checkout 一个新的热修复分支，如 `hotfix/auth`
2. 完成热修复
3. 推送分支，向 `main` 分支发起 Pull Request
4. 等待 CI 流水线全部通过，使用 squash merge 完成合并
5. 将热修复变更回灌到 `develop`（向 `develop` 发起 PR，使用 squash merge 完成合并）
