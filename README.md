# 原理
1. docker push 应用镜像之后, 收集下列文件, 使用 alpine 基础镜像, build 一个 hook 用的镜像, 将下列文件放入其中
  - 构建日志: /build.log
  - 构建结果: /mail.body (html格式)
  - git commit日志: /git_commit.txt
2. 设置 build-hook 项目, 推送不同 hook 镜像, 触发一个 webhook 发送到此进程: e.g.
  - harbor.example.com/build-hook/demo-app:test_20240630120000
  - harbor.example.com/build-hook/demo-other:test_20240630120001
3. 服务进程
- 处理 `/hook` 上下文请求, 发送 #2 生成的详情邮件
- 一次性检查, 如果在截至时间之前没有收到对应 app(`hook.apps`) 的webhook 请求, 则发送构建失败的邮件,
- 周期检查, 按照 cron 表达式定义进行周期性检查, 根据收到 hook 的情况进行统计, 发送不带附件的邮件
