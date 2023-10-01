export default class NotImplementedError extends Error {
  constructor(message?: string) {
    super(message);

    this.message = message || 'Not implemented';
    this.name = 'NotImplementedError';
  }
}
