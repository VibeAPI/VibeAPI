import fs from 'node:fs/promises';
import path from 'node:path';

const translations = {
  en: {
    仅根管理员可见: 'Root administrators only',
    '只有根管理员可以访问渠道；使用日志和流量分流中的渠道信息也会被隐藏。':
      'Only root administrators can access channels; channel details are also hidden from logs and traffic flow.',
    '只有根管理员可以查看和访问系统设置。':
      'Only root administrators can see and access system settings.',
  },
  'zh-CN': {
    仅根管理员可见: '仅根管理员可见',
    '只有根管理员可以访问渠道；使用日志和流量分流中的渠道信息也会被隐藏。':
      '只有根管理员可以访问渠道；使用日志和流量分流中的渠道信息也会被隐藏。',
    '只有根管理员可以查看和访问系统设置。':
      '只有根管理员可以查看和访问系统设置。',
  },
  'zh-TW': {
    仅根管理员可见: '僅根管理員可見',
    '只有根管理员可以访问渠道；使用日志和流量分流中的渠道信息也会被隐藏。':
      '只有根管理員可以存取渠道；使用記錄和流量分流中的渠道資訊也會被隱藏。',
    '只有根管理员可以查看和访问系统设置。':
      '只有根管理員可以查看和存取系統設定。',
  },
  fr: {
    仅根管理员可见: 'Administrateurs racine uniquement',
    '只有根管理员可以访问渠道；使用日志和流量分流中的渠道信息也会被隐藏。':
      'Seuls les administrateurs racine peuvent accéder aux canaux ; leurs informations sont également masquées dans les journaux et la répartition du trafic.',
    '只有根管理员可以查看和访问系统设置。':
      'Seuls les administrateurs racine peuvent voir et accéder aux paramètres système.',
  },
  ja: {
    仅根管理员可见: 'ルート管理者のみ表示',
    '只有根管理员可以访问渠道；使用日志和流量分流中的渠道信息也会被隐藏。':
      'チャネルにアクセスできるのはルート管理者のみです。ログとトラフィック分流でもチャネル情報は非表示になります。',
    '只有根管理员可以查看和访问系统设置。':
      'システム設定を表示してアクセスできるのはルート管理者のみです。',
  },
  ru: {
    仅根管理员可见: 'Только для корневых администраторов',
    '只有根管理员可以访问渠道；使用日志和流量分流中的渠道信息也会被隐藏。':
      'Только корневые администраторы могут управлять каналами; сведения о каналах также скрываются в журналах и распределении трафика.',
    '只有根管理员可以查看和访问系统设置。':
      'Только корневые администраторы могут видеть и открывать системные настройки.',
  },
  vi: {
    仅根管理员可见: 'Chỉ quản trị viên gốc',
    '只有根管理员可以访问渠道；使用日志和流量分流中的渠道信息也会被隐藏。':
      'Chỉ quản trị viên gốc mới có thể truy cập kênh; thông tin kênh cũng bị ẩn trong nhật ký và luồng lưu lượng.',
    '只有根管理员可以查看和访问系统设置。':
      'Chỉ quản trị viên gốc mới có thể xem và truy cập cài đặt hệ thống.',
  },
};

for (const [locale, entries] of Object.entries(translations)) {
  const filePath = path.resolve(`src/i18n/locales/${locale}.json`);
  const json = JSON.parse(await fs.readFile(filePath, 'utf8'));
  Object.assign(json.translation, entries);
  json.translation = Object.fromEntries(
    Object.entries(json.translation).sort(([a], [b]) => a.localeCompare(b)),
  );
  await fs.writeFile(filePath, `${JSON.stringify(json, null, 2)}\n`, 'utf8');
}
