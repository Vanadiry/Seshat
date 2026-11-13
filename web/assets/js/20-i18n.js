// 界面文案，弹窗提示在这里修改
var MSG = {
    // 配置提示
    configWarnBackendTitle: "自定义后端模式",
    configWarnBackendBody: "数据从自定义后端获取，部分功能按后端能力显示",
    configWarnFallbackTitle: "回退模式",
    configWarnFallbackBody: "本地缺失的数据会从回退端点读取",
    configWarnBothTitle: "自定义后端和回退模式",
    configWarnBothBody: "数据从自定义后端获取，本地缺失时从回退端点获取",
    configWarnFailedTitle: "自定义后端连接失败",
    configWarnFailedBody: "能力相关功能已禁用",
    // 管理操作
    btnCancel: "取消",
    importCollDone: function (n) {
        return "已同步 " + n + " 个条目到 tracker/user.json";
    },

    // 确认提示
    confirmUserFetch: function () {
        return "会拉取 " + window.USERNAME + " 的用户数据与收藏列表。";
    },
    confirmRebuildIndex: "会从本地 JSON 重建全部索引。",
    confirmRebuildAll: function () {
        var d = window.SESHAT_HOME || "~/.vSoft/Seshat";
        return "会删除 " + d + "/data 的全部数据，并完整重建。";
    },

    // 校验
    errInvalidTrackerName:
        "Tracker ID 不合法，仅允许大小写字母、数字、短横线和下划线。",
    errUserFetchFailed: "刷新用户数据失败：",
    errNoUsername:
        "未在配置中设置用户名 [user] username，无法刷新用户数据。",

    // 进度
    progressConnecting: "连接中",
    progressDone: "完成",

    // 信息栏
    infoboxGender: "性别",
    infoboxBloodType: "血型",
    infoboxBirthday: "生日",
    bloodTypeMap: { 1: "A", 2: "B", 3: "AB", 4: "O" },

    // 章节类型
    epTypes: {
        0: "本篇",
        1: "SP",
        2: "OP",
        3: "ED",
        4: "预告",
        5: "MAD",
        6: "其他"
    },
    // 条目类型，用于角色出演
    subjectTypes: { 1: "书籍", 2: "动画", 3: "音乐", 4: "游戏", 6: "三次元" },
    genderMap: { male: "男", female: "女" },
    careerMap: {
        actor: "演员",
        artist: "艺术家",
        illustrator: "插画家",
        producer: "制作人",
        seiyu: "声优",
        writer: "作家"
    }
};
