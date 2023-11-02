import { isValidUrl } from '@/utils/validate'
import ChildProcess from 'child_process'

const { exec } = ChildProcess
export function useOpenLink(url: string) {
  if (isValidUrl(url)) exec(`open ${url}`)
}
