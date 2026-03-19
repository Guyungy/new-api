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
import { Empty, Descriptions } from '@douyinfe/semi-ui';
import CardTable from '../../common/ui/CardTable';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { getLogsColumns } from './UsageLogsColumnDefs';

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
    hasExpandableRows,
    isAdminUser,
    billingDisplayMode,
    t,
    COLUMN_KEYS,
  } = logsData;

  // Get all columns
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

  // Filter columns based on visibility settings
  const getVisibleColumns = () => {
    return allColumns.filter((column) => visibleColumns[column.key]);
  };

  const visibleColumnsList = useMemo(() => {
    return getVisibleColumns();
  }, [visibleColumns, allColumns]);

  const tableColumns = useMemo(() => {
    return compactMode
      ? visibleColumnsList.map(({ fixed, ...rest }) => rest)
      : visibleColumnsList;
  }, [compactMode, visibleColumnsList]);

  const expandRowRender = (record, index) => {
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
        title: t('用户名称'),
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

  const isRankingView = isAdminUser && viewMode === VIEW_MODE.USER_RANKING;

  return (
    <CardTable
      columns={isRankingView ? rankingColumns : tableColumns}
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
