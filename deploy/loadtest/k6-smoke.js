// k6 冒烟压测脚本（只读接口，不产生业务数据）
//
// 前置：先人工登录取得 access token（限流策略下登录接口 10 次/分/IP，不参与压测）
//   k6 run -e K6_TOKEN=xxx -e BASE_URL=http://localhost:8080 deploy/loadtest/k6-smoke.js
//
// Windows PowerShell:
//   $env:K6_TOKEN="..."; $env:BASE_URL="http://localhost:8080"; k6 run deploy/loadtest/k6-smoke.js
import http from 'k6/http'
import { check, sleep } from 'k6'

const BASE = __ENV.BASE_URL || 'http://localhost:8080'
const TOKEN = __ENV.K6_TOKEN || ''

export const options = {
  stages: [
    { duration: '30s', target: 20 }, // 爬坡
    { duration: '1m', target: 20 }, // 稳态
    { duration: '20s', target: 0 }, // 回落
  ],
  thresholds: {
    http_req_failed: ['rate<0.01'], // 错误率 < 1%
    http_req_duration: ['p(95)<500'], // p95 < 500ms（本机 mock 链路基线）
  },
}

export default function () {
  // 公开只读接口（岗位列表）
  const jobs = http.get(`${BASE}/api/v1/jobs?page=1&page_size=20`)
  check(jobs, {
    'jobs status 200': (r) => r.status === 200,
    'jobs envelope ok': (r) => r.json('code') === 'SUCCESS',
  })

  // 鉴权接口（携带预取 token；未提供时跳过，不影响公开链路结论）
  if (TOKEN) {
    const me = http.get(`${BASE}/api/v1/me`, {
      headers: { Authorization: `Bearer ${TOKEN}` },
    })
    check(me, { 'me status 200': (r) => r.status === 200 })
  }

  // 指标端点自身不应成为瓶颈
  const metrics = http.get(`${BASE}/metrics`)
  check(metrics, { 'metrics status 200': (r) => r.status === 200 })

  sleep(1)
}
