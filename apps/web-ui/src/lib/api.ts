import axios from 'axios';

export const api = axios.create({ baseURL: '/api', timeout: 8000 });

api.interceptors.response.use(
  (response) => response,
  (error) => Promise.reject(new Error(error?.response?.data?.message ?? 'Request failed'))
);
