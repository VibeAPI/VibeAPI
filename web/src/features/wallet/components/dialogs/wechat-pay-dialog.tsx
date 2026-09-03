import { QRCodeSVG } from 'qrcode.react'
import { useTranslation } from 'react-i18next'

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

type WeChatPayDialogProps = {
  codeUrl: string
  onOpenChange: (open: boolean) => void
}

export function WeChatPayDialog(props: WeChatPayDialogProps) {
  const { t } = useTranslation()

  return (
    <Dialog open={Boolean(props.codeUrl)} onOpenChange={props.onOpenChange}>
      <DialogContent className='sm:max-w-sm'>
        <DialogHeader>
          <DialogTitle>{t('WeChat Pay')}</DialogTitle>
          <DialogDescription>
            {t('Scan the QR code with WeChat to complete payment.')}
          </DialogDescription>
        </DialogHeader>
        <div className='flex justify-center rounded-xl bg-white p-5'>
          {props.codeUrl && <QRCodeSVG value={props.codeUrl} size={224} />}
        </div>
        <p className='text-muted-foreground text-center text-sm'>
          {t('After payment, close this dialog and refresh your balance.')}
        </p>
      </DialogContent>
    </Dialog>
  )
}
