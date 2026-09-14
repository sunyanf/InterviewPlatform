-- 岗位分类种子数据
INSERT INTO job_categories (id, name, description, sort_order, created_at, updated_at) VALUES
    ('cat-backend', '后端开发', '服务端开发岗位，包括 Go、Java、Python 等', 1, NOW(), NOW()),
    ('cat-frontend', '前端开发', 'Web 前端开发岗位，包括 React、Vue 等', 2, NOW(), NOW()),
    ('cat-fullstack', '全栈开发', '前后端全栈开发岗位', 3, NOW(), NOW()),
    ('cat-algorithm', '算法工程师', '算法与数据结构相关岗位', 4, NOW(), NOW()),
    ('cat-data', '数据工程师', '数据开发、数据分析、数据科学岗位', 5, NOW(), NOW()),
    ('cat-devops', '运维/DevOps', '运维、SRE、DevOps 岗位', 6, NOW(), NOW()),
    ('cat-civil', '公务员', '公务员及结构化面试岗位', 7, NOW(), NOW())
ON CONFLICT (name) DO NOTHING;

-- 示例岗位
INSERT INTO jobs (id, category_id, title, description, requirements, skills, source, status, created_at, updated_at) VALUES
    ('job-go-senior', 'cat-backend', '高级 Go 后端工程师', '负责高并发后端服务开发',
     '["3年以上Go开发经验","熟悉分布式系统","有高并发系统经验"]'::jsonb,
     '["Go","MySQL","Redis","Kafka","微服务"]'::jsonb,
     'seed', 'active', NOW(), NOW()),
    ('job-react-frontend', 'cat-frontend', 'React 前端工程师', '负责 Web 前端开发',
     '["2年以上React经验","熟悉TypeScript","有组件库开发经验"]'::jsonb,
     '["React","TypeScript","Next.js","CSS"]'::jsonb,
     'seed', 'active', NOW(), NOW())
ON CONFLICT DO NOTHING;
