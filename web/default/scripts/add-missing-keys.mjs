import fs from 'node:fs/promises'
import path from 'node:path'

const LOCALES_DIR = path.resolve('src/i18n/locales')

function stableStringify(obj) {
  return `${JSON.stringify(obj, null, 2)}\n`
}

const key =
  'Please complete the bank transfer first, then upload a clear transfer receipt. The balance will be credited after manual review.'

const excludeAdministratorsTranslations = {
  en: 'Exclude administrators',
  zh: '排除管理员',
  'zh-TW': '排除管理員',
  fr: 'Exclure les administrateurs',
  ja: '管理者を除外',
  ru: 'Исключить администраторов',
  vi: 'Loại trừ quản trị viên',
}

const commonTranslations = {
  en: {
    Rejected: 'Rejected',
    'Reject Order': 'Reject Order',
    'Rejection reason': 'Rejection reason',
    'Enter rejection reason': 'Enter rejection reason',
    'Please provide a rejection reason': 'Please provide a rejection reason',
    'Order rejected successfully': 'Order rejected successfully',
    'Failed to reject order': 'Failed to reject order',
    'Confirm rejection': 'Confirm rejection',
    'The rejection reason will be visible to the user. This action does not credit their balance.':
      'The rejection reason will be visible to the user. This action does not credit their balance.',
    'Corporate contact phone': 'Corporate contact phone',
    'Corporate contact WeChat': 'Corporate contact WeChat',
    'Visible user groups': 'Visible user groups',
    'Leave all groups unselected to show corporate payment to every user group.':
      'Leave all groups unselected to show corporate payment to every user group.',
    'No user groups configured': 'No user groups configured',
    'Contact WeChat': 'Contact WeChat',
    'e.g., +86 138 0000 0000': 'e.g., +86 138 0000 0000',
    'e.g., finance_support': 'e.g., finance_support',
    Canceled: 'Canceled',
    'Withdraw Order': 'Withdraw Order',
    'Order withdrawn successfully': 'Order withdrawn successfully',
    'Failed to withdraw order': 'Failed to withdraw order',
    'Confirm withdrawal': 'Confirm withdrawal',
    'Withdraw this pending corporate payment order? The uploaded proof will remain available in the order history.':
      'Withdraw this pending corporate payment order? The uploaded proof will remain available in the order history.',
    'Root administrators only': 'Root administrators only',
    'Only root administrators can access channels; channel details are also hidden from logs and traffic flow.':
      'Only root administrators can access channels; channel details are also hidden from logs and traffic flow.',
    'Only root administrators can see and access system settings.':
      'Only root administrators can see and access system settings.',
  },
  zh: {
    Rejected: '已拒绝',
    'Reject Order': '拒绝订单',
    'Rejection reason': '拒绝原因',
    'Enter rejection reason': '请输入拒绝原因',
    'Please provide a rejection reason': '请填写拒绝原因',
    'Order rejected successfully': '订单已拒绝',
    'Failed to reject order': '拒绝订单失败',
    'Confirm rejection': '确认拒绝',
    'The rejection reason will be visible to the user. This action does not credit their balance.':
      '拒绝原因将对用户可见，此操作不会增加用户余额。',
    'Corporate contact phone': '对公联系电话',
    'Corporate contact WeChat': '对公联系微信',
    'Visible user groups': '可见用户分组',
    'Leave all groups unselected to show corporate payment to every user group.':
      '不选择任何分组时，对公支付将对所有用户分组显示。',
    'No user groups configured': '尚未配置用户分组',
    'Contact WeChat': '联系微信',
    'e.g., +86 138 0000 0000': '例如：+86 138 0000 0000',
    'e.g., finance_support': '例如：finance_support',
    Canceled: '已撤回',
    'Withdraw Order': '撤回订单',
    'Order withdrawn successfully': '订单已撤回',
    'Failed to withdraw order': '撤回订单失败',
    'Confirm withdrawal': '确认撤回',
    'Withdraw this pending corporate payment order? The uploaded proof will remain available in the order history.':
      '确定撤回这个待确认的对公支付订单吗？已上传的凭证仍会保留在订单记录中。',
    'Root administrators only': '仅根管理员可见',
    'Only root administrators can access channels; channel details are also hidden from logs and traffic flow.':
      '只有根管理员可以访问渠道；使用日志和流量分流中的渠道信息也会被隐藏。',
    'Only root administrators can see and access system settings.':
      '只有根管理员可以查看和访问系统设置。',
  },
  'zh-TW': {
    Rejected: '已拒絕',
    'Reject Order': '拒絕訂單',
    'Rejection reason': '拒絕原因',
    'Enter rejection reason': '請輸入拒絕原因',
    'Please provide a rejection reason': '請填寫拒絕原因',
    'Order rejected successfully': '訂單已拒絕',
    'Failed to reject order': '拒絕訂單失敗',
    'Confirm rejection': '確認拒絕',
    'The rejection reason will be visible to the user. This action does not credit their balance.':
      '拒絕原因將對使用者可見，此操作不會增加使用者餘額。',
    'Corporate contact phone': '對公聯絡電話',
    'Corporate contact WeChat': '對公聯絡微信',
    'Visible user groups': '可見使用者群組',
    'Leave all groups unselected to show corporate payment to every user group.':
      '不選擇任何群組時，對公支付將對所有使用者群組顯示。',
    'No user groups configured': '尚未設定使用者群組',
    'Contact WeChat': '聯絡微信',
    'e.g., +86 138 0000 0000': '例如：+86 138 0000 0000',
    'e.g., finance_support': '例如：finance_support',
    Canceled: '已撤回',
    'Withdraw Order': '撤回訂單',
    'Order withdrawn successfully': '訂單已撤回',
    'Failed to withdraw order': '撤回訂單失敗',
    'Confirm withdrawal': '確認撤回',
    'Withdraw this pending corporate payment order? The uploaded proof will remain available in the order history.':
      '確定撤回這個待確認的對公支付訂單嗎？已上傳的憑證仍會保留在訂單記錄中。',
    'Root administrators only': '僅根管理員可見',
    'Only root administrators can access channels; channel details are also hidden from logs and traffic flow.':
      '只有根管理員可以存取渠道；使用記錄和流量分流中的渠道資訊也會被隱藏。',
    'Only root administrators can see and access system settings.':
      '只有根管理員可以查看和存取系統設定。',
  },
  fr: {
    Rejected: 'Refusé',
    'Reject Order': 'Refuser la commande',
    'Rejection reason': 'Motif du refus',
    'Enter rejection reason': 'Saisissez le motif du refus',
    'Please provide a rejection reason': 'Veuillez indiquer un motif de refus',
    'Order rejected successfully': 'Commande refusée',
    'Failed to reject order': 'Échec du refus de la commande',
    'Confirm rejection': 'Confirmer le refus',
    'The rejection reason will be visible to the user. This action does not credit their balance.':
      'Le motif du refus sera visible par l’utilisateur. Cette action ne crédite pas son solde.',
    'Corporate contact phone': 'Téléphone du contact entreprise',
    'Corporate contact WeChat': 'WeChat du contact entreprise',
    'Visible user groups': 'Groupes d’utilisateurs visibles',
    'Leave all groups unselected to show corporate payment to every user group.':
      'Ne sélectionnez aucun groupe pour afficher le paiement entreprise à tous les groupes.',
    'No user groups configured': 'Aucun groupe d’utilisateurs configuré',
    'Contact WeChat': 'WeChat du contact',
    'e.g., +86 138 0000 0000': 'p. ex. +86 138 0000 0000',
    'e.g., finance_support': 'p. ex. finance_support',
    Canceled: 'Retiré',
    'Withdraw Order': 'Retirer la commande',
    'Order withdrawn successfully': 'Commande retirée',
    'Failed to withdraw order': 'Échec du retrait de la commande',
    'Confirm withdrawal': 'Confirmer le retrait',
    'Withdraw this pending corporate payment order? The uploaded proof will remain available in the order history.':
      'Retirer cette commande de paiement entreprise en attente ? Le justificatif restera disponible dans l’historique.',
    'Root administrators only': 'Administrateurs racine uniquement',
    'Only root administrators can access channels; channel details are also hidden from logs and traffic flow.':
      'Seuls les administrateurs racine peuvent accéder aux canaux ; leurs informations sont également masquées dans les journaux et la répartition du trafic.',
    'Only root administrators can see and access system settings.':
      'Seuls les administrateurs racine peuvent voir et accéder aux paramètres système.',
  },
  ja: {
    Rejected: '却下済み',
    'Reject Order': '注文を却下',
    'Rejection reason': '却下理由',
    'Enter rejection reason': '却下理由を入力してください',
    'Please provide a rejection reason': '却下理由を入力してください',
    'Order rejected successfully': '注文を却下しました',
    'Failed to reject order': '注文の却下に失敗しました',
    'Confirm rejection': '却下を確定',
    'The rejection reason will be visible to the user. This action does not credit their balance.':
      '却下理由はユーザーに表示されます。この操作では残高は追加されません。',
    'Corporate contact phone': '法人支払い連絡先電話番号',
    'Corporate contact WeChat': '法人支払い連絡先WeChat',
    'Visible user groups': '表示するユーザーグループ',
    'Leave all groups unselected to show corporate payment to every user group.':
      'グループを選択しない場合、法人支払いはすべてのユーザーグループに表示されます。',
    'No user groups configured': 'ユーザーグループが設定されていません',
    'Contact WeChat': '連絡先WeChat',
    'e.g., +86 138 0000 0000': '例：+86 138 0000 0000',
    'e.g., finance_support': '例：finance_support',
    Canceled: '撤回済み',
    'Withdraw Order': '注文を撤回',
    'Order withdrawn successfully': '注文を撤回しました',
    'Failed to withdraw order': '注文の撤回に失敗しました',
    'Confirm withdrawal': '撤回を確定',
    'Withdraw this pending corporate payment order? The uploaded proof will remain available in the order history.':
      'この確認待ちの法人支払い注文を撤回しますか？アップロード済みの証憑は注文履歴に残ります。',
    'Root administrators only': 'ルート管理者のみ表示',
    'Only root administrators can access channels; channel details are also hidden from logs and traffic flow.':
      'チャネルにアクセスできるのはルート管理者のみです。ログとトラフィック分流でもチャネル情報は非表示になります。',
    'Only root administrators can see and access system settings.':
      'システム設定を表示してアクセスできるのはルート管理者のみです。',
  },
  ru: {
    Rejected: 'Отклонено',
    'Reject Order': 'Отклонить заказ',
    'Rejection reason': 'Причина отклонения',
    'Enter rejection reason': 'Укажите причину отклонения',
    'Please provide a rejection reason': 'Укажите причину отклонения',
    'Order rejected successfully': 'Заказ отклонён',
    'Failed to reject order': 'Не удалось отклонить заказ',
    'Confirm rejection': 'Подтвердить отклонение',
    'The rejection reason will be visible to the user. This action does not credit their balance.':
      'Причина отклонения будет видна пользователю. Это действие не пополняет баланс.',
    'Corporate contact phone': 'Телефон для корпоративных платежей',
    'Corporate contact WeChat': 'WeChat для корпоративных платежей',
    'Visible user groups': 'Доступные группы пользователей',
    'Leave all groups unselected to show corporate payment to every user group.':
      'Не выбирайте группы, чтобы показать корпоративную оплату всем пользователям.',
    'No user groups configured': 'Группы пользователей не настроены',
    'Contact WeChat': 'WeChat для связи',
    'e.g., +86 138 0000 0000': 'например, +86 138 0000 0000',
    'e.g., finance_support': 'например, finance_support',
    Canceled: 'Отозвано',
    'Withdraw Order': 'Отозвать заказ',
    'Order withdrawn successfully': 'Заказ отозван',
    'Failed to withdraw order': 'Не удалось отозвать заказ',
    'Confirm withdrawal': 'Подтвердить отзыв',
    'Withdraw this pending corporate payment order? The uploaded proof will remain available in the order history.':
      'Отозвать этот ожидающий корпоративный платёж? Загруженный документ останется в истории заказов.',
    'Root administrators only': 'Только для корневых администраторов',
    'Only root administrators can access channels; channel details are also hidden from logs and traffic flow.':
      'Только корневые администраторы могут управлять каналами; сведения о каналах также скрываются в журналах и распределении трафика.',
    'Only root administrators can see and access system settings.':
      'Только корневые администраторы могут видеть и открывать системные настройки.',
  },
  vi: {
    Rejected: 'Đã từ chối',
    'Reject Order': 'Từ chối đơn hàng',
    'Rejection reason': 'Lý do từ chối',
    'Enter rejection reason': 'Nhập lý do từ chối',
    'Please provide a rejection reason': 'Vui lòng nhập lý do từ chối',
    'Order rejected successfully': 'Đã từ chối đơn hàng',
    'Failed to reject order': 'Không thể từ chối đơn hàng',
    'Confirm rejection': 'Xác nhận từ chối',
    'The rejection reason will be visible to the user. This action does not credit their balance.':
      'Người dùng sẽ thấy lý do từ chối. Thao tác này không cộng số dư.',
    'Corporate contact phone': 'Số điện thoại liên hệ doanh nghiệp',
    'Corporate contact WeChat': 'WeChat liên hệ doanh nghiệp',
    'Visible user groups': 'Nhóm người dùng được hiển thị',
    'Leave all groups unselected to show corporate payment to every user group.':
      'Không chọn nhóm nào để hiển thị thanh toán doanh nghiệp cho mọi nhóm người dùng.',
    'No user groups configured': 'Chưa cấu hình nhóm người dùng',
    'Contact WeChat': 'WeChat liên hệ',
    'e.g., +86 138 0000 0000': 'ví dụ: +86 138 0000 0000',
    'e.g., finance_support': 'ví dụ: finance_support',
    Canceled: 'Đã rút lại',
    'Withdraw Order': 'Rút lại đơn hàng',
    'Order withdrawn successfully': 'Đã rút lại đơn hàng',
    'Failed to withdraw order': 'Không thể rút lại đơn hàng',
    'Confirm withdrawal': 'Xác nhận rút lại',
    'Withdraw this pending corporate payment order? The uploaded proof will remain available in the order history.':
      'Rút lại đơn thanh toán doanh nghiệp đang chờ này? Chứng từ đã tải lên vẫn được lưu trong lịch sử đơn hàng.',
    'Root administrators only': 'Chỉ quản trị viên gốc',
    'Only root administrators can access channels; channel details are also hidden from logs and traffic flow.':
      'Chỉ quản trị viên gốc mới có thể truy cập kênh; thông tin kênh cũng bị ẩn trong nhật ký và luồng lưu lượng.',
    'Only root administrators can see and access system settings.':
      'Chỉ quản trị viên gốc mới có thể xem và truy cập cài đặt hệ thống.',
  },
}

const newKeys = {
  en: {
    [key]: key,
    ...commonTranslations.en,
  },
  zh: {
    [key]:
      '请先完成银行转账，然后上传清晰的转账凭证。人工审核通过后，余额将到账。',
    ...commonTranslations.zh,
  },
  'zh-TW': {
    [key]:
      '請先完成銀行轉帳，然後上傳清晰的轉帳憑證。人工審核通過後，餘額將入帳。',
    ...commonTranslations['zh-TW'],
  },
  fr: {
    [key]:
      'Effectuez d’abord le virement bancaire, puis téléversez un justificatif lisible. Le solde sera crédité après vérification manuelle.',
    ...commonTranslations.fr,
  },
  ja: {
    [key]:
      '先に銀行振込を完了し、鮮明な振込証憑をアップロードしてください。手動審査後に残高へ反映されます。',
    ...commonTranslations.ja,
  },
  ru: {
    [key]:
      'Сначала выполните банковский перевод, затем загрузите четкий документ, подтверждающий перевод. Баланс будет пополнен после ручной проверки.',
    ...commonTranslations.ru,
  },
  vi: {
    [key]:
      'Vui lòng hoàn tất chuyển khoản ngân hàng trước, sau đó tải lên chứng từ chuyển khoản rõ ràng. Số dư sẽ được cộng sau khi kiểm duyệt thủ công.',
    ...commonTranslations.vi,
  },
}

const goPayTranslations = {
  en: {
    'After payment, close this dialog and refresh your balance.':
      'After payment, close this dialog and refresh your balance.',
    'Alipay App ID': 'Alipay App ID',
    'Alipay application private key': 'Alipay application private key',
    'Alipay callback URL:': 'Alipay callback URL:',
    'Alipay public key': 'Alipay public key',
    'Alipay sandbox': 'Alipay sandbox',
    'Direct integration through go-pay using official merchant credentials.':
      'Direct integration through go-pay using official merchant credentials.',
    'Enable official Alipay': 'Enable official Alipay',
    'Enable official WeChat Pay': 'Enable official WeChat Pay',
    'Merchant API private key': 'Merchant API private key',
    'Merchant certificate serial number': 'Merchant certificate serial number',
    'Official Alipay and WeChat Pay': 'Official Alipay and WeChat Pay',
    'Scan the QR code with WeChat to complete payment.':
      'Scan the QR code with WeChat to complete payment.',
    'Use Alipay computer website payment.':
      'Use Alipay computer website payment.',
    'Use the Alipay sandbox gateway.': 'Use the Alipay sandbox gateway.',
    'Use WeChat Pay API v3 Native QR code payment.':
      'Use WeChat Pay API v3 Native QR code payment.',
    'Used to verify Alipay asynchronous notifications.':
      'Used to verify Alipay asynchronous notifications.',
    'Used to verify WeChat Pay API v3 responses and notifications.':
      'Used to verify WeChat Pay API v3 responses and notifications.',
    'WeChat App ID': 'WeChat App ID',
    'WeChat merchant ID': 'WeChat merchant ID',
    'WeChat Pay API v3 key': 'WeChat Pay API v3 key',
    'WeChat Pay callback URL:': 'WeChat Pay callback URL:',
    'WeChat platform certificate': 'WeChat platform certificate',
    'WeChat platform certificate serial number':
      'WeChat platform certificate serial number',
  },
  zh: {
    'After payment, close this dialog and refresh your balance.':
      '支付完成后，关闭此对话框并刷新余额。',
    'Alipay App ID': '支付宝 App ID',
    'Alipay application private key': '支付宝应用私钥',
    'Alipay callback URL:': '支付宝回调地址：',
    'Alipay public key': '支付宝公钥',
    'Alipay sandbox': '支付宝沙箱',
    'Direct integration through go-pay using official merchant credentials.':
      '通过 go-pay 使用官方商户凭据直接接入。',
    'Enable official Alipay': '启用官方支付宝',
    'Enable official WeChat Pay': '启用官方微信支付',
    'Merchant API private key': '商户 API 私钥',
    'Merchant certificate serial number': '商户证书序列号',
    'Official Alipay and WeChat Pay': '官方支付宝与微信支付',
    'Scan the QR code with WeChat to complete payment.':
      '请使用微信扫描二维码完成支付。',
    'Use Alipay computer website payment.': '使用支付宝电脑网站支付。',
    'Use the Alipay sandbox gateway.': '使用支付宝沙箱网关。',
    'Use WeChat Pay API v3 Native QR code payment.':
      '使用微信支付 API v3 Native 扫码支付。',
    'Used to verify Alipay asynchronous notifications.':
      '用于验证支付宝异步通知。',
    'Used to verify WeChat Pay API v3 responses and notifications.':
      '用于验证微信支付 API v3 响应和通知。',
    'WeChat App ID': '微信 App ID',
    'WeChat merchant ID': '微信支付商户号',
    'WeChat Pay API v3 key': '微信支付 API v3 密钥',
    'WeChat Pay callback URL:': '微信支付回调地址：',
    'WeChat platform certificate': '微信支付平台证书',
    'WeChat platform certificate serial number': '微信支付平台证书序列号',
  },
  'zh-TW': {
    'After payment, close this dialog and refresh your balance.':
      '付款完成後，請關閉此對話框並重新整理餘額。',
    'Alipay App ID': '支付寶 App ID',
    'Alipay application private key': '支付寶應用程式私鑰',
    'Alipay callback URL:': '支付寶回呼網址：',
    'Alipay public key': '支付寶公鑰',
    'Alipay sandbox': '支付寶沙箱',
    'Direct integration through go-pay using official merchant credentials.':
      '透過 go-pay 使用官方商戶憑證直接串接。',
    'Enable official Alipay': '啟用官方支付寶',
    'Enable official WeChat Pay': '啟用官方微信支付',
    'Merchant API private key': '商戶 API 私鑰',
    'Merchant certificate serial number': '商戶憑證序號',
    'Official Alipay and WeChat Pay': '官方支付寶與微信支付',
    'Scan the QR code with WeChat to complete payment.':
      '請使用微信掃描 QR Code 完成付款。',
    'Use Alipay computer website payment.': '使用支付寶電腦網站支付。',
    'Use the Alipay sandbox gateway.': '使用支付寶沙箱閘道。',
    'Use WeChat Pay API v3 Native QR code payment.':
      '使用微信支付 API v3 Native 掃碼支付。',
    'Used to verify Alipay asynchronous notifications.':
      '用於驗證支付寶非同步通知。',
    'Used to verify WeChat Pay API v3 responses and notifications.':
      '用於驗證微信支付 API v3 回應與通知。',
    'WeChat App ID': '微信 App ID',
    'WeChat merchant ID': '微信支付商戶號',
    'WeChat Pay API v3 key': '微信支付 API v3 金鑰',
    'WeChat Pay callback URL:': '微信支付回呼網址：',
    'WeChat platform certificate': '微信支付平台憑證',
    'WeChat platform certificate serial number': '微信支付平台憑證序號',
  },
  fr: {
    'After payment, close this dialog and refresh your balance.':
      'Après le paiement, fermez cette fenêtre et actualisez votre solde.',
    'Alipay App ID': 'ID d’application Alipay',
    'Alipay application private key': 'Clé privée de l’application Alipay',
    'Alipay callback URL:': 'URL de rappel Alipay :',
    'Alipay public key': 'Clé publique Alipay',
    'Alipay sandbox': 'Bac à sable Alipay',
    'Direct integration through go-pay using official merchant credentials.':
      'Intégration directe via go-pay avec les identifiants marchand officiels.',
    'Enable official Alipay': 'Activer Alipay officiel',
    'Enable official WeChat Pay': 'Activer WeChat Pay officiel',
    'Merchant API private key': 'Clé privée API du marchand',
    'Merchant certificate serial number':
      'Numéro de série du certificat marchand',
    'Official Alipay and WeChat Pay': 'Alipay et WeChat Pay officiels',
    'Scan the QR code with WeChat to complete payment.':
      'Scannez le QR code avec WeChat pour terminer le paiement.',
    'Use Alipay computer website payment.':
      'Utiliser le paiement Alipay pour site web sur ordinateur.',
    'Use the Alipay sandbox gateway.': 'Utiliser la passerelle de test Alipay.',
    'Use WeChat Pay API v3 Native QR code payment.':
      'Utiliser le paiement QR Native de l’API v3 WeChat Pay.',
    'Used to verify Alipay asynchronous notifications.':
      'Utilisée pour vérifier les notifications asynchrones Alipay.',
    'Used to verify WeChat Pay API v3 responses and notifications.':
      'Utilisé pour vérifier les réponses et notifications de l’API v3 WeChat Pay.',
    'WeChat App ID': 'ID d’application WeChat',
    'WeChat merchant ID': 'ID marchand WeChat',
    'WeChat Pay API v3 key': 'Clé API v3 WeChat Pay',
    'WeChat Pay callback URL:': 'URL de rappel WeChat Pay :',
    'WeChat platform certificate': 'Certificat de plateforme WeChat',
    'WeChat platform certificate serial number':
      'Numéro de série du certificat de plateforme WeChat',
  },
  ja: {
    'After payment, close this dialog and refresh your balance.':
      '支払い後、このダイアログを閉じて残高を更新してください。',
    'Alipay App ID': 'Alipay アプリ ID',
    'Alipay application private key': 'Alipay アプリケーション秘密鍵',
    'Alipay callback URL:': 'Alipay コールバック URL：',
    'Alipay public key': 'Alipay 公開鍵',
    'Alipay sandbox': 'Alipay サンドボックス',
    'Direct integration through go-pay using official merchant credentials.':
      '公式加盟店認証情報を使用して go-pay 経由で直接連携します。',
    'Enable official Alipay': '公式 Alipay を有効化',
    'Enable official WeChat Pay': '公式 WeChat Pay を有効化',
    'Merchant API private key': '加盟店 API 秘密鍵',
    'Merchant certificate serial number': '加盟店証明書シリアル番号',
    'Official Alipay and WeChat Pay': '公式 Alipay / WeChat Pay',
    'Scan the QR code with WeChat to complete payment.':
      'WeChat で QR コードを読み取り、支払いを完了してください。',
    'Use Alipay computer website payment.':
      'Alipay の PC ウェブサイト決済を使用します。',
    'Use the Alipay sandbox gateway.':
      'Alipay サンドボックスゲートウェイを使用します。',
    'Use WeChat Pay API v3 Native QR code payment.':
      'WeChat Pay API v3 Native QR コード決済を使用します。',
    'Used to verify Alipay asynchronous notifications.':
      'Alipay の非同期通知の検証に使用します。',
    'Used to verify WeChat Pay API v3 responses and notifications.':
      'WeChat Pay API v3 の応答と通知の検証に使用します。',
    'WeChat App ID': 'WeChat アプリ ID',
    'WeChat merchant ID': 'WeChat Pay 加盟店 ID',
    'WeChat Pay API v3 key': 'WeChat Pay API v3 キー',
    'WeChat Pay callback URL:': 'WeChat Pay コールバック URL：',
    'WeChat platform certificate': 'WeChat プラットフォーム証明書',
    'WeChat platform certificate serial number':
      'WeChat プラットフォーム証明書シリアル番号',
  },
  ru: {
    'After payment, close this dialog and refresh your balance.':
      'После оплаты закройте это окно и обновите баланс.',
    'Alipay App ID': 'ID приложения Alipay',
    'Alipay application private key': 'Закрытый ключ приложения Alipay',
    'Alipay callback URL:': 'URL обратного вызова Alipay:',
    'Alipay public key': 'Открытый ключ Alipay',
    'Alipay sandbox': 'Песочница Alipay',
    'Direct integration through go-pay using official merchant credentials.':
      'Прямая интеграция через go-pay с официальными данными продавца.',
    'Enable official Alipay': 'Включить официальный Alipay',
    'Enable official WeChat Pay': 'Включить официальный WeChat Pay',
    'Merchant API private key': 'Закрытый ключ API продавца',
    'Merchant certificate serial number': 'Серийный номер сертификата продавца',
    'Official Alipay and WeChat Pay': 'Официальные Alipay и WeChat Pay',
    'Scan the QR code with WeChat to complete payment.':
      'Отсканируйте QR-код в WeChat, чтобы завершить оплату.',
    'Use Alipay computer website payment.':
      'Использовать оплату Alipay для компьютерного сайта.',
    'Use the Alipay sandbox gateway.': 'Использовать тестовый шлюз Alipay.',
    'Use WeChat Pay API v3 Native QR code payment.':
      'Использовать Native QR-оплату WeChat Pay API v3.',
    'Used to verify Alipay asynchronous notifications.':
      'Используется для проверки асинхронных уведомлений Alipay.',
    'Used to verify WeChat Pay API v3 responses and notifications.':
      'Используется для проверки ответов и уведомлений WeChat Pay API v3.',
    'WeChat App ID': 'ID приложения WeChat',
    'WeChat merchant ID': 'ID продавца WeChat',
    'WeChat Pay API v3 key': 'Ключ WeChat Pay API v3',
    'WeChat Pay callback URL:': 'URL обратного вызова WeChat Pay:',
    'WeChat platform certificate': 'Сертификат платформы WeChat',
    'WeChat platform certificate serial number':
      'Серийный номер сертификата платформы WeChat',
  },
  vi: {
    'After payment, close this dialog and refresh your balance.':
      'Sau khi thanh toán, hãy đóng hộp thoại này và làm mới số dư.',
    'Alipay App ID': 'ID ứng dụng Alipay',
    'Alipay application private key': 'Khóa riêng ứng dụng Alipay',
    'Alipay callback URL:': 'URL callback Alipay:',
    'Alipay public key': 'Khóa công khai Alipay',
    'Alipay sandbox': 'Môi trường thử nghiệm Alipay',
    'Direct integration through go-pay using official merchant credentials.':
      'Tích hợp trực tiếp qua go-pay bằng thông tin thương nhân chính thức.',
    'Enable official Alipay': 'Bật Alipay chính thức',
    'Enable official WeChat Pay': 'Bật WeChat Pay chính thức',
    'Merchant API private key': 'Khóa riêng API của thương nhân',
    'Merchant certificate serial number': 'Số sê-ri chứng thư thương nhân',
    'Official Alipay and WeChat Pay': 'Alipay và WeChat Pay chính thức',
    'Scan the QR code with WeChat to complete payment.':
      'Quét mã QR bằng WeChat để hoàn tất thanh toán.',
    'Use Alipay computer website payment.':
      'Sử dụng thanh toán trang web máy tính Alipay.',
    'Use the Alipay sandbox gateway.': 'Sử dụng cổng thử nghiệm Alipay.',
    'Use WeChat Pay API v3 Native QR code payment.':
      'Sử dụng thanh toán mã QR Native của WeChat Pay API v3.',
    'Used to verify Alipay asynchronous notifications.':
      'Dùng để xác minh thông báo bất đồng bộ của Alipay.',
    'Used to verify WeChat Pay API v3 responses and notifications.':
      'Dùng để xác minh phản hồi và thông báo WeChat Pay API v3.',
    'WeChat App ID': 'ID ứng dụng WeChat',
    'WeChat merchant ID': 'ID thương nhân WeChat',
    'WeChat Pay API v3 key': 'Khóa WeChat Pay API v3',
    'WeChat Pay callback URL:': 'URL callback WeChat Pay:',
    'WeChat platform certificate': 'Chứng thư nền tảng WeChat',
    'WeChat platform certificate serial number':
      'Số sê-ri chứng thư nền tảng WeChat',
  },
}

for (const [locale, translations] of Object.entries(goPayTranslations)) {
  Object.assign(newKeys[locale], translations)
}

for (const [locale, translation] of Object.entries(
  excludeAdministratorsTranslations
)) {
  newKeys[locale]['Exclude administrators'] = translation
}

const modelCatalogTranslations = {
  en: {
    'Add a source...': 'Add a source...',
    'Billing unit': 'Billing unit',
    'Choose from icons already used by your model library.':
      'Choose from icons already used by your model library.',
    'Cost in USD for each second of usage.':
      'Cost in USD for each second of usage.',
    'Describe the model capabilities and intended use.':
      'Describe the model capabilities and intended use.',
    'Failed to save model metadata': 'Failed to save model metadata',
    'Model description': 'Model description',
    'Model icon': 'Model icon',
    'No icon': 'No icon',
    'Only applies to fixed-price models.':
      'Only applies to fixed-price models.',
    'Optional labels such as official, community, or provider.':
      'Optional labels such as official, community, or provider.',
    'Per Second': 'Per Second',
    'Per second': 'Per second',
    'Price per second': 'Price per second',
    'Shown on the public model catalog.': 'Shown on the public model catalog.',
    Sources: 'Sources',
    'The fixed price is charged for each request.':
      'The fixed price is charged for each request.',
    'The fixed price is charged for each second.':
      'The fixed price is charged for each second.',
    'per second': 'per second',
    second: 'second',
  },
  zh: {
    'Add a source...': '添加来源...',
    'Billing unit': '计费单位',
    'Choose from icons already used by your model library.':
      '从模型库已使用的图标中选择。',
    'Cost in USD for each second of usage.': '每秒使用费用（美元）。',
    'Describe the model capabilities and intended use.':
      '描述模型能力和适用场景。',
    'Failed to save model metadata': '保存模型信息失败',
    'Model description': '模型描述',
    'Model icon': '模型图标',
    'No icon': '无图标',
    'Only applies to fixed-price models.': '仅适用于固定价格模型。',
    'Optional labels such as official, community, or provider.':
      '可选标签，例如官方、社区或供应商。',
    'Per Second': '按秒计费',
    'Per second': '按秒',
    'Price per second': '每秒价格',
    'Shown on the public model catalog.': '显示在公开模型广场中。',
    Sources: '来源',
    'The fixed price is charged for each request.': '固定价格按每次请求收取。',
    'The fixed price is charged for each second.': '固定价格按每秒收取。',
    'per second': '每秒',
    second: '秒',
  },
  'zh-TW': {
    'Add a source...': '新增來源...',
    'Billing unit': '計費單位',
    'Choose from icons already used by your model library.':
      '從模型庫已使用的圖示中選擇。',
    'Cost in USD for each second of usage.': '每秒使用費用（美元）。',
    'Describe the model capabilities and intended use.':
      '描述模型能力和適用場景。',
    'Failed to save model metadata': '儲存模型資訊失敗',
    'Model description': '模型描述',
    'Model icon': '模型圖示',
    'No icon': '無圖示',
    'Only applies to fixed-price models.': '僅適用於固定價格模型。',
    'Optional labels such as official, community, or provider.':
      '可選標籤，例如官方、社群或供應商。',
    'Per Second': '按秒計費',
    'Per second': '按秒',
    'Price per second': '每秒價格',
    'Shown on the public model catalog.': '顯示在公開模型廣場中。',
    Sources: '來源',
    'The fixed price is charged for each request.': '固定價格按每次請求收取。',
    'The fixed price is charged for each second.': '固定價格按每秒收取。',
    'per second': '每秒',
    second: '秒',
  },
  fr: {
    'Add a source...': 'Ajouter une source...',
    'Billing unit': 'Unité de facturation',
    'Choose from icons already used by your model library.':
      'Choisissez parmi les icônes déjà utilisées dans votre bibliothèque.',
    'Cost in USD for each second of usage.':
      'Coût en USD pour chaque seconde d’utilisation.',
    'Describe the model capabilities and intended use.':
      'Décrivez les capacités et les usages prévus du modèle.',
    'Failed to save model metadata':
      'Échec de l’enregistrement des informations du modèle',
    'Model description': 'Description du modèle',
    'Model icon': 'Icône du modèle',
    'No icon': 'Aucune icône',
    'Only applies to fixed-price models.':
      'S’applique uniquement aux modèles à prix fixe.',
    'Optional labels such as official, community, or provider.':
      'Libellés facultatifs, par exemple officiel, communauté ou fournisseur.',
    'Per Second': 'Par seconde',
    'Per second': 'Par seconde',
    'Price per second': 'Prix par seconde',
    'Shown on the public model catalog.':
      'Affiché dans le catalogue public des modèles.',
    Sources: 'Sources',
    'The fixed price is charged for each request.':
      'Le prix fixe est facturé pour chaque requête.',
    'The fixed price is charged for each second.':
      'Le prix fixe est facturé pour chaque seconde.',
    'per second': 'par seconde',
    second: 'seconde',
  },
  ja: {
    'Add a source...': 'ソースを追加...',
    'Billing unit': '課金単位',
    'Choose from icons already used by your model library.':
      'モデルライブラリで使用中のアイコンから選択します。',
    'Cost in USD for each second of usage.': '1秒あたりの利用料金（USD）。',
    'Describe the model capabilities and intended use.':
      'モデルの機能と用途を説明します。',
    'Failed to save model metadata': 'モデル情報の保存に失敗しました',
    'Model description': 'モデルの説明',
    'Model icon': 'モデルアイコン',
    'No icon': 'アイコンなし',
    'Only applies to fixed-price models.': '固定価格モデルにのみ適用されます。',
    'Optional labels such as official, community, or provider.':
      '公式、コミュニティ、プロバイダーなどの任意ラベル。',
    'Per Second': '秒単位',
    'Per second': '秒単位',
    'Price per second': '1秒あたりの価格',
    'Shown on the public model catalog.': '公開モデルカタログに表示されます。',
    Sources: '提供元',
    'The fixed price is charged for each request.':
      '固定価格はリクエストごとに課金されます。',
    'The fixed price is charged for each second.':
      '固定価格は1秒ごとに課金されます。',
    'per second': '秒あたり',
    second: '秒',
  },
  ru: {
    'Add a source...': 'Добавить источник...',
    'Billing unit': 'Единица тарификации',
    'Choose from icons already used by your model library.':
      'Выберите значок из уже используемых в библиотеке моделей.',
    'Cost in USD for each second of usage.':
      'Стоимость в USD за каждую секунду использования.',
    'Describe the model capabilities and intended use.':
      'Опишите возможности и назначение модели.',
    'Failed to save model metadata': 'Не удалось сохранить данные модели',
    'Model description': 'Описание модели',
    'Model icon': 'Значок модели',
    'No icon': 'Без значка',
    'Only applies to fixed-price models.':
      'Применяется только к моделям с фиксированной ценой.',
    'Optional labels such as official, community, or provider.':
      'Необязательные метки: официальный, сообщество или поставщик.',
    'Per Second': 'Посекундно',
    'Per second': 'За секунду',
    'Price per second': 'Цена за секунду',
    'Shown on the public model catalog.':
      'Отображается в общедоступном каталоге моделей.',
    Sources: 'Источники',
    'The fixed price is charged for each request.':
      'Фиксированная цена взимается за каждый запрос.',
    'The fixed price is charged for each second.':
      'Фиксированная цена взимается за каждую секунду.',
    'per second': 'за секунду',
    second: 'секунда',
  },
  vi: {
    'Add a source...': 'Thêm nguồn...',
    'Billing unit': 'Đơn vị tính phí',
    'Choose from icons already used by your model library.':
      'Chọn từ các biểu tượng đã dùng trong thư viện mô hình.',
    'Cost in USD for each second of usage.':
      'Chi phí bằng USD cho mỗi giây sử dụng.',
    'Describe the model capabilities and intended use.':
      'Mô tả khả năng và mục đích sử dụng của mô hình.',
    'Failed to save model metadata': 'Không thể lưu thông tin mô hình',
    'Model description': 'Mô tả mô hình',
    'Model icon': 'Biểu tượng mô hình',
    'No icon': 'Không có biểu tượng',
    'Only applies to fixed-price models.':
      'Chỉ áp dụng cho mô hình có giá cố định.',
    'Optional labels such as official, community, or provider.':
      'Nhãn tùy chọn như chính thức, cộng đồng hoặc nhà cung cấp.',
    'Per Second': 'Theo giây',
    'Per second': 'Theo giây',
    'Price per second': 'Giá mỗi giây',
    'Shown on the public model catalog.':
      'Hiển thị trong danh mục mô hình công khai.',
    Sources: 'Nguồn',
    'The fixed price is charged for each request.':
      'Giá cố định được tính cho mỗi yêu cầu.',
    'The fixed price is charged for each second.':
      'Giá cố định được tính cho mỗi giây.',
    'per second': 'mỗi giây',
    second: 'giây',
  },
}

for (const [locale, translations] of Object.entries(modelCatalogTranslations)) {
  Object.assign(newKeys[locale], translations)
}

const usageLogTimeRangeTranslations = {
  en: {
    'Day before yesterday': 'Day before yesterday',
    'Last 1 hour': 'Last 1 hour',
    'Last 7 days': 'Last 7 days',
    'Last 12 hours': 'Last 12 hours',
    'Last 24 hours': 'Last 24 hours',
    'Last 30 days': 'Last 30 days',
    'This month': 'This month',
    Yesterday: 'Yesterday',
  },
  zh: {
    'Day before yesterday': '前天',
    'Last 1 hour': '近 1 小时',
    'Last 7 days': '近 7 天',
    'Last 12 hours': '近 12 小时',
    'Last 24 hours': '近 24 小时',
    'Last 30 days': '近 30 天',
    'This month': '本月',
    Yesterday: '昨天',
  },
  'zh-TW': {
    'Day before yesterday': '前天',
    'Last 1 hour': '近 1 小時',
    'Last 7 days': '近 7 天',
    'Last 12 hours': '近 12 小時',
    'Last 24 hours': '近 24 小時',
    'Last 30 days': '近 30 天',
    'This month': '本月',
    Yesterday: '昨天',
  },
  fr: {
    'Day before yesterday': 'Avant-hier',
    'Last 1 hour': 'Dernière heure',
    'Last 7 days': '7 derniers jours',
    'Last 12 hours': '12 dernières heures',
    'Last 24 hours': '24 dernières heures',
    'Last 30 days': '30 derniers jours',
    'This month': 'Ce mois-ci',
    Yesterday: 'Hier',
  },
  ja: {
    'Day before yesterday': '一昨日',
    'Last 1 hour': '直近1時間',
    'Last 7 days': '直近7日間',
    'Last 12 hours': '直近12時間',
    'Last 24 hours': '直近24時間',
    'Last 30 days': '直近30日間',
    'This month': '今月',
    Yesterday: '昨日',
  },
  ru: {
    'Day before yesterday': 'Позавчера',
    'Last 1 hour': 'Последний час',
    'Last 7 days': 'Последние 7 дней',
    'Last 12 hours': 'Последние 12 часов',
    'Last 24 hours': 'Последние 24 часа',
    'Last 30 days': 'Последние 30 дней',
    'This month': 'В этом месяце',
    Yesterday: 'Вчера',
  },
  vi: {
    'Day before yesterday': 'Hôm kia',
    'Last 1 hour': '1 giờ qua',
    'Last 7 days': '7 ngày qua',
    'Last 12 hours': '12 giờ qua',
    'Last 24 hours': '24 giờ qua',
    'Last 30 days': '30 ngày qua',
    'This month': 'Tháng này',
    Yesterday: 'Hôm qua',
  },
}

for (const [locale, translations] of Object.entries(
  usageLogTimeRangeTranslations
)) {
  Object.assign(newKeys[locale], translations)
}

const modelIconTranslations = {
  en: {
    'Application icons': 'Application icons',
    'Choose a provider or model icon from the library.':
      'Choose a provider or model icon from the library.',
    'Custom icons': 'Custom icons',
    'Model icons': 'Model icons',
    'Provider icons': 'Provider icons',
  },
  zh: {
    'Application icons': '应用图标',
    'Choose a provider or model icon from the library.':
      '从图标库中选择供应商或模型图标。',
    'Custom icons': '自定义图标',
    'Model icons': '模型图标',
    'Provider icons': '供应商图标',
  },
  'zh-TW': {
    'Application icons': '應用程式圖示',
    'Choose a provider or model icon from the library.':
      '從圖示庫中選擇供應商或模型圖示。',
    'Custom icons': '自訂圖示',
    'Model icons': '模型圖示',
    'Provider icons': '供應商圖示',
  },
  fr: {
    'Application icons': "Icônes d'applications",
    'Choose a provider or model icon from the library.':
      'Choisissez une icône de fournisseur ou de modèle dans la bibliothèque.',
    'Custom icons': 'Icônes personnalisées',
    'Model icons': 'Icônes de modèles',
    'Provider icons': 'Icônes de fournisseurs',
  },
  ja: {
    'Application icons': 'アプリケーションアイコン',
    'Choose a provider or model icon from the library.':
      'ライブラリからプロバイダーまたはモデルのアイコンを選択します。',
    'Custom icons': 'カスタムアイコン',
    'Model icons': 'モデルアイコン',
    'Provider icons': 'プロバイダーアイコン',
  },
  ru: {
    'Application icons': 'Значки приложений',
    'Choose a provider or model icon from the library.':
      'Выберите значок поставщика или модели из библиотеки.',
    'Custom icons': 'Пользовательские значки',
    'Model icons': 'Значки моделей',
    'Provider icons': 'Значки поставщиков',
  },
  vi: {
    'Application icons': 'Biểu tượng ứng dụng',
    'Choose a provider or model icon from the library.':
      'Chọn biểu tượng nhà cung cấp hoặc mô hình từ thư viện.',
    'Custom icons': 'Biểu tượng tùy chỉnh',
    'Model icons': 'Biểu tượng mô hình',
    'Provider icons': 'Biểu tượng nhà cung cấp',
  },
}

for (const [locale, translations] of Object.entries(modelIconTranslations)) {
  Object.assign(newKeys[locale], translations)
}

async function main() {
  let totalApplied = 0

  for (const [locale, translations] of Object.entries(newKeys)) {
    const filePath = path.join(LOCALES_DIR, `${locale}.json`)
    const json = JSON.parse(await fs.readFile(filePath, 'utf8'))
    let count = 0

    for (const [translationKey, value] of Object.entries(translations)) {
      if (json.translation[translationKey] !== value) {
        json.translation[translationKey] = value
        count++
      }
    }

    if (count > 0) {
      json.translation = Object.fromEntries(
        Object.entries(json.translation).sort(([a], [b]) => a.localeCompare(b))
      )
      await fs.writeFile(filePath, stableStringify(json), 'utf8')
    }

    console.log(`${locale}: ${count} translations applied`)
    totalApplied += count
  }

  console.log(`\nTotal: ${totalApplied} translations applied`)
}

main().catch((error) => {
  console.error(error)
  process.exitCode = 1
})
