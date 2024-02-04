import { stringify } from 'qs';
import merge from 'deepmerge';
import axios from 'axios';
import { LegacyDataProvider } from 'ra-core';
import {
  GET_LIST,
  GET_ONE,
  CREATE,
  UPDATE,
  DELETE,
  GET_MANY,
  GET_MANY_REFERENCE,
} from './actions';

import defaultSettings from './default-settings';
import NotImplementedError from './errors';
import init from './initializer';

interface LooseObject {
  [key: string]: any
}

// Set HTTP interceptors.
init();

/**
 * Maps react-admin queries to a JSONAPI REST API
 *
 * @param apiUrl the base URL for the JSONAPI
 * @param userSettings Settings to configure this client.
 *
 * @returns {Promise} the Promise for a data response
 */
function dataProviderFactory(apiUrl: string, userSettings: object): LegacyDataProvider {
  /**
   * @param type Request type, e.g GET_LIST
   * @param resource Resource name, e.g. "posts"
   * @param payload Request parameters. Depends on the request type
   */
  return async (type: string, resource: string, params: any): Promise<any> => {
    let url: string = '';
    const settings = merge(defaultSettings, userSettings);

    const options : LooseObject = {
      headers: settings.headers,
    };

    switch (type) {
      case GET_LIST: {
        const { page, perPage } = params.pagination;

        // Create query with pagination params.
        const query : LooseObject = {
          'page[number]': page,
          'page[size]': perPage,
        };

        // Add all filter params to query.
        Object.keys(params.filter || {}).forEach((key) => {
          query[`filter[${key}]`] = params.filter[key];
        });

        // Add sort parameter
        if (params.sort && params.sort.field) {
          const prefix = params.sort.order === 'ASC' ? '' : '-';
          query.sort = `${prefix}${params.sort.field}`;
        }

        url = `${apiUrl}/${resource}?${stringify(query)}`;
        break;
      }

      case GET_ONE:
        url = `${apiUrl}/${resource}/${params.id}`;
        break;

      case CREATE:
        url = `${apiUrl}/${resource}`;
        options.method = 'POST';
        options.data = JSON.stringify({
          data: { type: resource, attributes: params.data },
        });
        break;

      case UPDATE: {
        url = `${apiUrl}/${resource}/${params.id}`;

        const attributes = params.data;
        delete attributes.id;

        const data = {
          data: {
            id: params.id,
            type: resource,
            attributes,
          },
        };

        options.method = settings.updateMethod;
        options.data = JSON.stringify(data);
        break;
      }

      case DELETE:
        url = `${apiUrl}/${resource}/${params.id}`;
        options.method = 'DELETE';
        break;

      case GET_MANY: {
        const query = stringify({
          [`filter[${settings.getManyKey}]`]: params.ids,
        }, { arrayFormat: settings.arrayFormat });

        url = `${apiUrl}/${resource}?${query}`;
        break;
      }

      case GET_MANY_REFERENCE: {
        const { page, perPage } = params.pagination;

        // Create query with pagination params.
        const query : LooseObject = {
          'page[number]': page,
          'page[size]': perPage,
        };

        // Add all filter params to query.
        Object.keys(params.filter || {}).forEach((key) => {
          query[`filter[${key}]`] = params.filter[key];
        });

        // Add the reference id to the filter params.
        query[`filter[${params.target}]`] = params.id;

        // Add sort parameter
        if (params.sort && params.sort.field) {
          const prefix = params.sort.order === 'ASC' ? '' : '-';
          query.sort = `${prefix}${params.sort.field}`;
        }

        url = `${apiUrl}/${resource}?${stringify(query)}`;
        break;
      }

      default:
        throw new NotImplementedError(`Unsupported Data Provider request type ${type}`);
    }

    const response = await axios({ url, ...options });
    let total;
    // For all collection requests get the total count.
    if ([GET_LIST, GET_MANY, GET_MANY_REFERENCE].includes(type)) {
      // When metadata and the 'total' setting is provided try
      // to get the total count.
      if (response.data.meta && settings.total) {
        total = response.data.meta[settings.total];
      }

      // Use the length of the data array as a fallback.
      total = total || response.data.data.length;
    }
    switch (type) {
      case GET_MANY:
      case GET_LIST: {
        return {
          data: response.data.data.map((value_2: LooseObject) => ({
            id: value_2.id,
            ...value_2.attributes,
          })),
          total,
        };
      }

      case GET_MANY_REFERENCE: {
        return {
          data: response.data.data.map((value_3: LooseObject) => ({
            id: value_3.id,
            ...value_3.attributes,
          })),
          total,
        };
      }

      case GET_ONE: {
        const { id, attributes: attributes1 } = response.data.data;

        return {
          data: {
            id, ...attributes1,
          },
        };
      }

      case CREATE: {
        const { id: id1, attributes: attributes2 } = response.data.data;

        return {
          data: {
            id1, ...attributes2,
          },
        };
      }

      case UPDATE: {
        const { id: id2, attributes: attributes3 } = response.data.data;

        return {
          data: {
            id2, ...attributes3,
          },
        };
      }

      case DELETE: {
        return {
          data: { id: params.id },
        };
      }

      default:
        throw new NotImplementedError(`Unsupported Data Provider request type ${type}`);
    }
  };
}

export default dataProviderFactory;
