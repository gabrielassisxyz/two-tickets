// No title has a media row, so no response carries mediaInfo.
export default class Media {
  static async getMedia() {
    return undefined;
  }

  static async getRelatedMedia() {
    return [];
  }
}
