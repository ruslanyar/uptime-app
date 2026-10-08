import { AuthError } from './api';
import type { Coordination, SessionEvent } from './session';

// Share only events and a failure flag; users and tokens must stay out of browser storage.
/** Unavailable coordination APIs or storage produce AuthError('unsupported'). */
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
  /** Uncertainty survives reloads and is shared across tabs on this origin. */
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
  /** Serializes tabs on this origin; rejects if unsupported or the task fails. */
  lock = async <T>(task: () => Promise<T>): Promise<T> => {
    if (!navigator.locks || typeof BroadcastChannel === 'undefined')
      return Promise.reject(new AuthError('unsupported'));
    return await navigator.locks.request('uptime-auth-session', task);
  };
  broadcast = (event: SessionEvent) => {
    this.channel?.postMessage(event);
  };
}
