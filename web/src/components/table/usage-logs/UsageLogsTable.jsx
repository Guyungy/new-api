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

import React, { useMemo } from 'react';
import { Empty, Descriptions, Tag, Typography } from '@douyinfe/semi-ui';
import CardTable from '../../common/ui/CardTable';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { getLogsColumns } from './UsageLogsColumnDefs';
import { getLogOther, renderQuota } from '../../../helpers';

const INTERCEPT_MODE_COLOR = {
  normal: 'green',
  ignore: 'red',
  inject: 'blue',
  replace: 'orange',
};

const LogsTable = (logsData) => {
  const {
    logs,
    userRanking,
    viewMode,
    VIEW_MODE,
    expandData,
    loading,
    activePage,
    pageSize,
    logCount,
    compactMode,
    visibleColumns,
    handlePageChange,
    handlePageSizeChange,
    copyText,
    showUserInfoFunc,
    openChannelAffinityUsageCacheModal,
    setInterceptUsernameQuick,
    getFormValues,
    hasExpandableRows,
    isAdminUser,
    billingDisplayMode,
    t,
    COLUMN_KEYS,
  } = logsData;

  const allColumns = useMemo(() => {
    return getLogsColumns({
      t,
      COLUMN_KEYS,
      copyText,
      showUserInfoFunc,
      openChannelAffinityUsageCacheModal,
      isAdminUser,
      billingDisplayMode,
    });
  }, [
    t,
    COLUMN_KEYS,
    copyText,
    showUserInfoFunc,
    openChannelAffinityUsageCacheModal,
    isAdminUser,
    billingDisplayMode,
  ]);

  const currentInterceptUsername = getFormValues().username || '';

  const visibleColumnsList = useMemo(() => {
    return allColumns.filter((column) => visibleColumns[column.key]);
  }, [visibleColumns, allColumns]);

  const tableColumns = useMemo(() => {
    return compactMode
      ? visibleColumnsList.map(({ fixed, ...rest }) => rest)
      : visibleColumnsList;
  }, [compactMode, visibleColumnsList]);

  const expandRowRender = (record) => {
    return <Descriptions data={expandData[record.key]} />;
  };

  const rankingColumns = useMemo(
    () => [
      {
        title: t('排名'),
        dataIndex: 'rank',
        key: 'rank',
        width: 80,
        sorter: (a, b) => Number(a.rank || 0) - Number(b.rank || 0),
      },
      {
        title: t('用户 ID'),
        dataIndex: 'user_id',
        key: 'user_id',
        width: 100,
        sorter: (a, b) => Number(a.user_id || 0) - Number(b.user_id || 0),
      },
      {
        title: t('用户名'),
        dataIndex: 'username',
        key: 'username',
        width: 220,
        sorter: (a, b) =>
          String(a.username || '').localeCompare(String(b.username || '')),
      },
      {
        title: t('消耗额度'),
        dataIndex: 'quota',
        key: 'quota',
        width: 160,
        sorter: (a, b) => Number(a.quota || 0) - Number(b.quota || 0),
        render: (value) => Number(value || 0).toLocaleString(),
      },
      {
        title: t('折算金额'),
        dataIndex: 'quota',
        key: 'amount',
        width: 140,
        sorter: (a, b) => Number(a.quota || 0) - Number(b.quota || 0),
        render: (value) => renderQuota(Number(value || 0)),
      },
      {
        title: t('总 Tokens'),
        dataIndex: 'tokens',
        key: 'tokens',
        width: 160,
        sorter: (a, b) => Number(a.tokens || 0) - Number(b.tokens || 0),
        render: (value) => Number(value || 0).toLocaleString(),
      },
      {
        title: t('请求数'),
        dataIndex: 'request_count',
        key: 'request_count',
        width: 120,
        sorter: (a, b) =>
          Number(a.request_count || 0) - Number(b.request_count || 0),
        render: (value) => Number(value || 0).toLocaleString(),
      },
    ],
    [t],
  );

  const interceptColumns = useMemo(
    () => [
      {
        title: t('时间'),
        dataIndex: 'created_at',
        key: 'created_at',
        width: 180,
        render: (_, record) => record.timestamp2string || '-',
      },
      ...(isAdminUser
        ? [
            {
              title: t('用户名'),
              dataIndex: 'username',
              key: 'username',
              width: 160,
              render: (value) => (
                <Tag
                  color={currentInterceptUsername === value ? 'blue' : 'grey'}
                  shape='circle'
                  style={{ cursor: 'pointer' }}
                  onClick={() => setInterceptUsernameQuick(value)}
                >
                  {value || '-'}
                </Tag>
              ),
            },
          ]
        : []),
      {
        title: t('请求模式'),
        dataIndex: 'other',
        key: 'intercept_mode',
        width: 120,
        render: (_, record) => {
          const other = getLogOther(record.other);
          const mode = String(
            other?.request_audit_mode || other?.intercept_mode || '-',
          );
          return (
            <Tag color={INTERCEPT_MODE_COLOR[mode] || 'grey'} shape='circle'>
              {mode}
            </Tag>
          );
        },
      },
      {
        title: t('命中关键词'),
        dataIndex: 'other',
        key: 'matched_keywords',
        width: 180,
        render: (_, record) => {
          const other = getLogOther(record.other);
          const mode = String(
            other?.request_audit_mode || other?.intercept_mode || '',
          );
          const keywords = Array.isArray(other?.matched_keywords)
            ? other.matched_keywords.filter(Boolean)
            : [];
          if (keywords.length === 0) {
            return <span>{mode === 'normal' ? '-' : t('全部匹配')}</span>;
          }
          return (
            <Typography.Paragraph
              ellipsis={{
                rows: 2,
                showTooltip: {
                  type: 'popover',
                  opts: { style: { width: 240 } },
                },
              }}
              style={{ maxWidth: 180, marginBottom: 0 }}
            >
              {keywords.join(', ')}
            </Typography.Paragraph>
          );
        },
      },
      {
        title: t('模型上下文'),
        dataIndex: 'other',
        key: 'request_text',
        width: 420,
        render: (_, record) => {
          const other = getLogOther(record.other);
          const requestText = other?.request_text || record.content || '-';
          return (
            <Typography.Paragraph
              ellipsis={{
                rows: 3,
                showTooltip: {
                  type: 'popover',
                  opts: { style: { width: 420 } },
                },
              }}
              style={{ maxWidth: 420, marginBottom: 0, whiteSpace: 'pre-wrap' }}
            >
              {requestText}
            </Typography.Paragraph>
          );
        },
      },
      {
        title: t('模型回复'),
        dataIndex: 'other',
        key: 'response_text',
        width: 420,
        render: (_, record) => {
          const other = getLogOther(record.other);
          const responseText = other?.response_text || '-';
          return (
            <Typography.Paragraph
              ellipsis={{
                rows: 3,
                showTooltip: {
                  type: 'popover',
                  opts: { style: { width: 420 } },
                },
              }}
              style={{ maxWidth: 420, marginBottom: 0, whiteSpace: 'pre-wrap' }}
            >
              {responseText}
            </Typography.Paragraph>
          );
        },
      },
      {
        title: t('模型名称'),
        dataIndex: 'model_name',
        key: 'model_name',
        width: 180,
      },
      {
        title: t('Audit ID'),
        dataIndex: 'other',
        key: 'audit_id',
        width: 220,
        render: (_, record) => getLogOther(record.other)?.audit_id || '-',
      },
      {
        title: t('会话 ID'),
        dataIndex: 'other',
        key: 'session_id',
        width: 220,
        render: (_, record) => getLogOther(record.other)?.session_id || '-',
      },
      {
        title: t('对话 ID'),
        dataIndex: 'other',
        key: 'conversation_id',
        width: 220,
        render: (_, record) =>
          getLogOther(record.other)?.conversation_id || '-',
      },
      {
        title: t('父请求 ID'),
        dataIndex: 'other',
        key: 'parent_request_id',
        width: 220,
        render: (_, record) =>
          getLogOther(record.other)?.parent_request_id || '-',
      },
      {
        title: t('Request ID'),
        dataIndex: 'request_id',
        key: 'request_id',
        width: 180,
      },
    ],
    [currentInterceptUsername, getFormValues, isAdminUser, setInterceptUsernameQuick, t],
  );

  const isRankingView = isAdminUser && viewMode === VIEW_MODE.USER_RANKING;
  const isInterceptView =
    isAdminUser && viewMode === VIEW_MODE.INTERCEPT_DETAILS;

  return (
    <CardTable
      columns={
        isRankingView
          ? rankingColumns
          : isInterceptView
            ? interceptColumns
            : tableColumns
      }
      {...(!isRankingView && hasExpandableRows() && {
        expandedRowRender: expandRowRender,
        expandRowByClick: true,
        rowExpandable: (record) =>
          expandData[record.key] && expandData[record.key].length > 0,
      })}
      dataSource={isRankingView ? userRanking : logs}
      rowKey='key'
      loading={loading}
      scroll={compactMode ? undefined : { x: 'max-content' }}
      className='rounded-xl overflow-hidden'
      size='small'
      empty={
        <Empty
          image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
          darkModeImage={
            <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
          }
          description={t('搜索无结果')}
          style={{ padding: 30 }}
        />
      }
      pagination={{
        currentPage: activePage,
        pageSize: pageSize,
        total: logCount,
        pageSizeOptions: [10, 20, 50, 100],
        showSizeChanger: true,
        onPageSizeChange: (size) => {
          handlePageSizeChange(size);
        },
        onPageChange: handlePageChange,
      }}
      hidePagination={true}
    />
  );
};

export default LogsTable;
