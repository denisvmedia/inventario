import React, { useEffect, useState } from 'react';
import api from '../api/api';
import AddAreaForm from './AddAreaForm';

interface Area {
  id: string;
  name: string;
}

function AreaList() {
  const [areas, setAreas] = useState<Area[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    const fetchAreas = async () => {
      try {
        const response = await api.get('/areas');
        setAreas(response.data.data.items);
        setIsLoading(false);
      } catch (error) {
        setError('Error fetching areas');
        setIsLoading(false);
      }
    };

    fetchAreas();
  }, []);

  if (isLoading) {
    return <div>Loading...</div>;
  }

  if (error) {
    return <div>{error}</div>;
  }

  return (
    <div>
      <h1>Area List</h1>
      <AddAreaForm />
      {' '}
      {/* Add the form component */}
      {areas?.length > 0 ? (
        <ul>
          {areas.map((area) => (
            <li key={area.id}>{area.name}</li>
          ))}
        </ul>
      ) : (
        <div>No areas found.</div>
      )}
    </div>
  );
}

export default AreaList;
