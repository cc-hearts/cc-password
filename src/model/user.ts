import { encodeMd5 } from '@/utils/crypto'
import { getModelInstance } from './init'

async function findUser(user: { username: string; password: string }) {
  const userModel = getModelInstance('user')!
  const { username, password } = user
  return await userModel.findFirst({
    select: {
      username: true,
      password: true,
    },
    where: {
      username,
      password: encodeMd5(password),
    },
  })
}

export { findUser }
