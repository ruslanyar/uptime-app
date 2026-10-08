import {
  AuthAPI,
  AuthError,
  type Credentials,
  type Profile,
  type User,
} from './api';

export type SessionState = {
  status: 'loading' | 'authenticated' | 'anonymous' | 'error';
  user?: User;
  error?: unknown;
};
export type SessionEvent = 'changing' | 'changed' | 'logout' | 'uncertain';
export type Coordination = {
  lock: <T>(task: () => Promise<T>) => Promise<T>;
  broadcast: (event: SessionEvent) => void;
  isUncertain: () => boolean;
  setUncertain: (value: boolean) => void;
};
const initial: SessionState = { status: 'loading' };

export class Session {
  private state = initial;
  private listeners = new Set<() => void>();
  // In-flight requests must not restore a user after a newer session change.
  private generation = 0;
  private checking?: Promise<void>;
  private changing = false;
  private uncertain = false;
  constructor(
    private api: AuthAPI,
    private coordination: Coordination,
  ) {}
  snapshot = () => this.state;
  serverSnapshot = () => initial;
  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  };
  private set(state: SessionState) {
    this.state = state;
    this.listeners.forEach((listener) => listener());
  }
  fail(error: unknown) {
    this.generation++;
    this.set({ status: 'error', error });
  }
  receive(event: SessionEvent) {
    if (this.changing) return;
    if (event === 'uncertain') this.uncertain = true;
    this.generation++;
    this.checking = undefined;
    if (event === 'logout') {
      this.uncertain = false;
      this.set({ status: 'anonymous' });
    } else {
      this.set(initial);
      if (event === 'changed' || event === 'uncertain') void this.check();
    }
  }
  /** Coalesces checks and stores errors in state; explicit recovery permits uncertain refresh. */
  check = (explicit = false): Promise<void> => {
    if (this.changing) return Promise.resolve();
    if (this.checking) return this.checking;
    if (explicit) this.uncertain = false;
    const version = this.generation;
    const active = () => version === this.generation;
    const run = async () => {
      try {
        let current: User | undefined;
        try {
          current = await this.api.call('me');
        } catch (error) {
          if (!(error instanceof AuthError) || error.status !== 401)
            throw error;
          if (!active()) return;
          current = await this.coordination.lock(async () => {
            if (!active()) return;
            try {
              // Another tab may have refreshed while we waited for the lock.
              return await this.api.call('me');
            } catch (error) {
              if (!(error instanceof AuthError) || error.status !== 401)
                throw error;
            }
            if (!active()) return;
            if (explicit) {
              this.uncertain = false;
              this.coordination.setUncertain(false);
            }
            // A lost response may have consumed the refresh token; retrying can revoke the session.
            if (this.uncertain || this.coordination.isUncertain())
              throw new AuthError('uncertain');
            try {
              await this.api.call('refresh');
            } catch (error) {
              if (!(error instanceof AuthError) || error.kind !== 'http') {
                this.uncertain = true;
                this.coordination.setUncertain(true);
                this.coordination.broadcast('uncertain');
              }
              throw error;
            }
            const refreshed = await this.api.call('me');
            this.coordination.broadcast('changed');
            return refreshed;
          });
        }
        if (active() && current && explicit) {
          // Clear shared uncertainty only after checking cookies under the mutation lock.
          current = await this.coordination.lock(async () => {
            if (!active()) return;
            const verified = await this.api.call('me');
            this.coordination.setUncertain(false);
            this.uncertain = false;
            return verified;
          });
        }
        if (active() && current)
          this.set({ status: 'authenticated', user: current });
      } catch (error) {
        if (active())
          this.set(
            error instanceof AuthError && error.status === 401
              ? { status: 'anonymous' }
              : { status: 'error', error },
          );
      }
    };
    const promise = run().finally(() => {
      if (this.checking === promise) this.checking = undefined;
    });
    this.checking = promise;
    return promise;
  };
  /** Rejects on failure and updates state; overlapping local mutations are ignored. */
  async mutate(
    action: 'login' | 'register' | 'logout' | 'profile',
    values?: Credentials | Profile,
  ) {
    if (this.changing) return;
    if (action === 'profile') {
      // Renew expired access before the write so the mutation never needs a retry.
      await this.check();
      if (this.state.status !== 'authenticated')
        throw this.state.error ?? new AuthError('http', 401);
      if (this.changing) return;
    }
    this.changing = true;
    const version = ++this.generation;
    this.checking = undefined;
    this.set(initial);
    try {
      await this.coordination.lock(async () => {
        this.coordination.broadcast('changing');
        try {
          await this.api.call(action, values);
          this.uncertain = false;
          this.coordination.setUncertain(false);
          if (action === 'logout') {
            if (version === this.generation) this.set({ status: 'anonymous' });
            this.coordination.broadcast('logout');
          } else {
            let current;
            try {
              // A successful POST does not prove the browser accepted its cookies.
              current = await this.api.call('me');
            } catch (error) {
              if (error instanceof AuthError && error.status === 401)
                throw new AuthError('cookies');
              throw error;
            }
            if (version === this.generation)
              this.set({ status: 'authenticated', user: current });
            this.coordination.broadcast('changed');
          }
        } catch (error) {
          // The write may have committed; other tabs must not automatically rotate cookies.
          if (!(error instanceof AuthError) || error.kind !== 'http') {
            this.uncertain = true;
            this.coordination.setUncertain(true);
            this.coordination.broadcast('uncertain');
          } else this.coordination.broadcast('changed');
          throw error;
        }
      });
    } catch (error) {
      if (version === this.generation) this.set({ status: 'error', error });
      throw error;
    } finally {
      this.changing = false;
    }
  }
}
