/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React from 'react';
import { Button, Form } from '@douyinfe/semi-ui';
import { IconSearch } from '@douyinfe/semi-icons';

import { DATE_RANGE_PRESETS } from '../../../constants/console.constants';

const LogsFilters = ({
  formInitValues,
  setFormApi,
  refresh,
  setShowColumnSelector,
  formApi,
  setLogType,
  setViewMode,
  viewMode,
  VIEW_MODE,
  loading,
  isAdminUser,
  showViewModeSelector = true,
  t,
}) => {
  const isRankingView = viewMode === VIEW_MODE.USER_RANKING;
  const isInterceptView = viewMode === VIEW_MODE.INTERCEPT_DETAILS;
  const isLogDetailsView = viewMode === VIEW_MODE.LOGS;

  return (
    <Form
      initValues={formInitValues}
      getFormApi={(api) => setFormApi(api)}
      onSubmit={refresh}
      allowEmpty={true}
      autoComplete='off'
      layout='vertical'
      trigger='change'
      stopValidateWithError={false}
    >
      <div className='flex flex-col gap-2'>
        <div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-2'>
          <div className='col-span-1 lg:col-span-2'>
            <Form.DatePicker
              field='dateRange'
              className='w-full'
              type='dateTimeRange'
              placeholder={[t('开始时间'), t('结束时间')]}
              showClear
              pure
              size='small'
              presets={DATE_RANGE_PRESETS.map((preset) => ({
                text: t(preset.text),
                start: preset.start(),
                end: preset.end(),
              }))}
            />
          </div>

          <Form.Input
            field='token_name'
            prefix={<IconSearch />}
            placeholder={t('令牌名称')}
            showClear
            pure
            size='small'
          />

          <Form.Input
            field='model_name'
            prefix={<IconSearch />}
            placeholder={t('模型名称')}
            showClear
            pure
            size='small'
          />

          <Form.Input
            field='group'
            prefix={<IconSearch />}
            placeholder={t('分组')}
            showClear
            pure
            size='small'
          />

          <Form.Input
            field='request_id'
            prefix={<IconSearch />}
            placeholder={t('Request ID')}
            showClear
            pure
            size='small'
          />

          {isAdminUser && isInterceptView && (
            <>
              <Form.Input
                field='audit_id'
                prefix={<IconSearch />}
                placeholder={t('Audit ID')}
                showClear
                pure
                size='small'
              />
              <Form.Input
                field='session_id'
                prefix={<IconSearch />}
                placeholder={t('会话 ID')}
                showClear
                pure
                size='small'
              />
              <Form.Input
                field='conversation_id'
                prefix={<IconSearch />}
                placeholder={t('对话 ID')}
                showClear
                pure
                size='small'
              />
              <Form.Input
                field='parent_request_id'
                prefix={<IconSearch />}
                placeholder={t('父请求 ID')}
                showClear
                pure
                size='small'
              />
              <Form.Input
                field='intercept_keyword'
                prefix={<IconSearch />}
                placeholder={t('拦截关键词')}
                showClear
                pure
                size='small'
              />
              <Form.Input
                field='context_keyword'
                prefix={<IconSearch />}
                placeholder={t('搜索上下文')}
                showClear
                pure
                size='small'
              />
            </>
          )}

          {isAdminUser && showViewModeSelector && (
            <Form.Select
              field='viewMode'
              placeholder={t('视图')}
              pure
              size='small'
              onChange={(value) => {
                setViewMode(value || VIEW_MODE.LOGS);
                setTimeout(() => {
                  refresh();
                }, 0);
              }}
            >
              <Form.Select.Option value={VIEW_MODE.LOGS}>
                {t('日志明细')}
              </Form.Select.Option>
              <Form.Select.Option value={VIEW_MODE.USER_RANKING}>
                {t('用户用量排行')}
              </Form.Select.Option>
              <Form.Select.Option value={VIEW_MODE.INTERCEPT_DETAILS}>
                {t('请求明细')}
              </Form.Select.Option>
            </Form.Select>
          )}

          {isAdminUser && (
            <>
              <Form.Input
                field='channel'
                prefix={<IconSearch />}
                placeholder={t('渠道 ID')}
                showClear
                pure
                size='small'
              />
              <Form.Input
                field='username'
                prefix={<IconSearch />}
                placeholder={t('用户名')}
                showClear
                pure
                size='small'
              />
            </>
          )}

          {isAdminUser && isRankingView && (
            <>
              <Form.Select
                field='sort_by'
                placeholder={t('排序字段')}
                pure
                size='small'
                onChange={() => {
                  setTimeout(() => {
                    refresh();
                  }, 0);
                }}
              >
                <Form.Select.Option value='quota'>
                  {t('消耗额度')}
                </Form.Select.Option>
                <Form.Select.Option value='tokens'>
                  {t('总 Tokens')}
                </Form.Select.Option>
                <Form.Select.Option value='request_count'>
                  {t('请求数')}
                </Form.Select.Option>
              </Form.Select>

              <Form.Select
                field='sort_order'
                placeholder={t('排序方向')}
                pure
                size='small'
                onChange={() => {
                  setTimeout(() => {
                    refresh();
                  }, 0);
                }}
              >
                <Form.Select.Option value='desc'>
                  {t('降序')}
                </Form.Select.Option>
                <Form.Select.Option value='asc'>
                  {t('升序')}
                </Form.Select.Option>
              </Form.Select>
            </>
          )}
        </div>

        <div className='flex flex-col sm:flex-row justify-between items-start sm:items-center gap-3'>
          {isLogDetailsView && (
            <div className='w-full sm:w-auto'>
              <Form.Select
                field='logType'
                placeholder={t('日志类型')}
                className='w-full sm:w-auto min-w-[120px]'
                showClear
                pure
                onChange={() => {
                  setTimeout(() => {
                    refresh();
                  }, 0);
                }}
                size='small'
              >
                <Form.Select.Option value='0'>{t('全部')}</Form.Select.Option>
                <Form.Select.Option value='1'>{t('充值')}</Form.Select.Option>
                <Form.Select.Option value='2'>{t('消费')}</Form.Select.Option>
                <Form.Select.Option value='3'>{t('管理')}</Form.Select.Option>
                <Form.Select.Option value='4'>{t('系统')}</Form.Select.Option>
                <Form.Select.Option value='5'>{t('错误')}</Form.Select.Option>
                <Form.Select.Option value='6'>{t('退款')}</Form.Select.Option>
              </Form.Select>
            </div>
          )}

          <div className='flex gap-2 w-full sm:w-auto justify-end'>
            <Button
              type='tertiary'
              htmlType='submit'
              loading={loading}
              size='small'
            >
              {t('查询')}
            </Button>
            <Button
              type='tertiary'
              onClick={() => {
                if (formApi) {
                  formApi.reset();
                  setLogType(0);
                  setTimeout(() => {
                    refresh();
                  }, 100);
                }
              }}
              size='small'
            >
              {t('重置')}
            </Button>
            {isLogDetailsView && (
              <Button
                type='tertiary'
                onClick={() => setShowColumnSelector(true)}
                size='small'
              >
                {t('列设置')}
              </Button>
            )}
            {isInterceptView && (
              <Button type='tertiary' disabled size='small'>
                {t('已按请求拦截日志过滤')}
              </Button>
            )}
          </div>
        </div>
      </div>
    </Form>
  );
};

export default LogsFilters;
