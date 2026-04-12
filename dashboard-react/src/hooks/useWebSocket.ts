import { useEffect } from 'react'
import { omegaWS } from '../services/ws'
import { useAppStore } from '../store/useAppStore'

export function useWebSocket() {
  const updateTick = useAppStore((s) => s.updateTick)
  const setWsStatus = useAppStore((s) => s.setWsStatus)

  useEffect(() => {
    omegaWS.connect()
    let pingTs = 0

    const unsub = omegaWS.subscribe((data) => {
      const type = data.type as string

      if (type === '__connected__') {
        setWsStatus(true, 0)
        pingTs = Date.now()
      } else if (type === '__disconnected__') {
        setWsStatus(false, 0)
      } else if (type === 'tick') {
        updateTick({
          symbol:    data.symbol as string,
          price:     data.price  as number,
          bid:       data.bid    as number,
          ask:       data.ask    as number,
          volume:    data.volume as number,
          change_24h: data.change_24h as number,
          ts:        data.ts    as number,
        })
      } else if (type === 'ping') {
        const latency = Date.now() - pingTs
        setWsStatus(true, latency)
        pingTs = Date.now()
        omegaWS.send({ type: 'pong' })
      }
    })

    return () => {
      unsub()
      omegaWS.disconnect()
    }
  }, [updateTick, setWsStatus])
}
