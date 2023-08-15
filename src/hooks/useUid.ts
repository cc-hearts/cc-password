import { useProfile } from "@/storage";

export function useUid() {
  const { profile } = useProfile()
  const uid = profile.value?.uid
  if (!uid) throw new Error('uid is not defined')
  return profile.value.uid
}