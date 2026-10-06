import { AuthError } from './api';
import type { Coordination, SessionEvent } from './session';

export class BrowserCoordination implements Coordination {
  private channel?: BroadcastChannel;
  open(receive: (event: SessionEvent) => void) {
    if (!navigator.locks || typeof BroadcastChannel === 'undefined')
      throw new AuthError('unsupported');
    this.isUncertain();
    this.channel = new BroadcastChannel('uptime-auth-session');
    this.channel.onmessage = ({ data }) => {
      if (['changing', 'changed', 'logout', 'uncertain'].includes(data))
        receive(data);
    };
  }
  // Only a non-secret failure flag is shared; no users or tokens enter storage.
  isUncertain = () => {
    try {
      return localStorage.getItem('uptime-auth-uncertain') === '1';
    } catch {
      throw new AuthError('unsupported');
    }
  };
  setUncertain = (value: boolean) => {
    try {
      if (value) localStorage.setItem('uptime-auth-uncertain', '1');
      else localStorage.removeItem('uptime-auth-uncertain');
    } catch {
      throw new AuthError('unsupported');
    }
  };
  close() {
    this.channel?.close();
    this.channel = undefined;
  }
  lock = async <T>(task: () => Promise<T>): Promise<T> => {
    if (!navigator.locks || typeof BroadcastChannel === 'undefined')
      return Promise.reject(new AuthError('unsupported'));
    return await navigator.locks.request('uptime-auth-session', task);
  };
  broadcast = (event: SessionEvent) => {
    this.channel?.postMessage(event);
  };
}
